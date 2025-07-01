package eventconsumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/event"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go/jetstream"
)

type signConsumer struct {
	node          *mpc.Node
	durablePubsub messaging.DurablePubsub

	mpcThreshold       int
	signingResultQueue messaging.MessageQueue
	signingSub         messaging.Subscription
	identityStore      identity.Store

	activeSessions map[string]time.Time // Maps "walletID-txID" to creation time
	sessionsLock   sync.RWMutex
}

func NewSignConsumer(
	node *mpc.Node,
	durablePubsub messaging.DurablePubsub,
	mpcThreshold int,
	signingResultQueue messaging.MessageQueue,
	identityStore identity.Store,
) *signConsumer {
	return &signConsumer{
		node:               node,
		durablePubsub:      durablePubsub,
		mpcThreshold:       mpcThreshold,
		signingResultQueue: signingResultQueue,
		identityStore:      identityStore,
	}
}

func (sc *signConsumer) Start(ctx context.Context) error {
	// Subscribe to signing events
	sub, err := sc.durablePubsub.Subscribe(sc.handleTxSigningEvent)
	if err != nil {
		return err
	}
	sc.signingSub = sub

	logger.Info("SigningConsumer: Subscribed to signing events")

	// Block until context cancellation.
	<-ctx.Done()
	return nil
}

func (sc *signConsumer) Close() {
	if sc.signingSub != nil {
		err := sc.signingSub.Unsubscribe()
		if err != nil {
			logger.Error("Failed to unsubscribe from signing events", err)
		}
	}

	sc.sessionsLock.Lock()
	defer sc.sessionsLock.Unlock()
	sc.activeSessions = make(map[string]time.Time)
	logger.Info("SigningConsumer: Closed and unsubscribed from signing events")
}

func (sc *signConsumer) handleTxSigningEvent(jsMsg jetstream.Msg) {
	raw := jsMsg.Data()
	var msg types.SignTxMessage
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		logger.Error("Failed to unmarshal signing message", err)
		jsMsg.Ack()
		return
	}

	err = sc.identityStore.VerifyInitiatorMessage(&msg)
	if err != nil {
		logger.Error("Failed to verify initiator message", err)
		jsMsg.Nak()
		return
	}

	logger.Info(
		"Rsceived signing event",
		"waleltID",
		msg.WalletID,
		"type",
		msg.KeyType,
		"tx",
		msg.TxID,
		"Id",
		sc.node.ID(),
	)

	// Chsck for duplicate session and track if new
	if sc.checkDuplicateSession(msg.WalletID, msg.TxID) {
		jsMsg.Term()
		return
	}

	var session mpc.ISigningSession
	switch msg.KeyType {
	case types.KeyTypeSecp256k1:
		session, err = sc.node.CreateSigningSession(
			msg.WalletID,
			msg.TxID,
			msg.NetworkInternalCode,
			sc.mpcThreshold,
			sc.signingResultQueue,
		)
	case types.KeyTypeEd25519:
		session, err = sc.node.CreateEDDSASigningSession(
			msg.WalletID,
			msg.TxID,
			msg.NetworkInternalCode,
			sc.mpcThreshold,
			sc.signingResultQueue,
		)

	}

	if err != nil {
		sc.handleSigningSessionError(
			msg.WalletID,
			msg.TxID,
			msg.NetworkInternalCode,
			err,
			"Failed to create signing session",
			jsMsg,
		)
		return
	}

	txBigInt := new(big.Int).SetBytes(msg.Tx)
	err = session.Init(txBigInt)
	if err != nil {
		if errors.Is(err, mpc.ErrNotEnoughParticipants) {
			logger.Info("RETRY LATER: Not enough participants to sign")
			//Return for retry later
			return
		}
		sc.handleSigningSessionError(
			msg.WalletID,
			msg.TxID,
			msg.NetworkInternalCode,
			err,
			"Failed to init signing session",
			jsMsg,
		)
		return
	}

	// Mark session as already processed
	sc.addSession(msg.WalletID, msg.TxID)

	ctx, done := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-session.ErrChan():
				if err != nil {
					sc.handleSigningSessionError(
						msg.WalletID,
						msg.TxID,
						msg.NetworkInternalCode,
						err,
						"Failed to sign tx",
						jsMsg,
					)
					return
				}
			}
		}
	}()

	session.ListenToIncomingMessageAsync()
	// TODO: use consul distributed lock here, only sign after all nodes has already completed listing to incoming message async
	// The purpose of the sleep is to be ensuring that the node has properly set up its message listeners
	// before it starts the signing process. If the signing process starts sending messages before other nodes
	// have set up their listeners, those messages might be missed, potentially causing the signing process to fail.
	// One solution:
	// The messaging includes mschanisms for dirsct point-to-point communication (in point2point.go).
	// The nodes could explicitly coordinate through request-response patterns before starting signing
	time.Sleep(1 * time.Second)

	onSuccess := func(data []byte) {
		done()
		jsMsg.Ack()
	}
	go session.Sign(onSuccess)
}

func (sc *signConsumer) handleSigningSessionError(walletID, txID, NetworkInternalCode string, err error, errMsg string, jsMsg jetstream.Msg) {
	logger.Error("Signing session error", err, "walletID", walletID, "txID", txID, "error", errMsg)
	signingResult := event.SigningResultEvent{
		ResultType:          event.SigningResultTypeError,
		NetworkInternalCode: NetworkInternalCode,
		WalletID:            walletID,
		TxID:                txID,
		ErrorReason:         errMsg,
	}

	signingResultBytes, err := json.Marshal(signingResult)
	if err != nil {
		logger.Error("Failed to marshal signing result event", err)
		return
	}

	jsMsg.Ack()
	err = sc.signingResultQueue.Enqueue(event.SigningResultCompleteTopic, signingResultBytes, &messaging.EnqueueOptions{
		IdempotententKey: txID,
	})
	if err != nil {
		logger.Error("Failed to publish signing result event", err)
		return
	}
}

func (sc *signConsumer) checkDuplicateSession(walletID, txID string) bool {
	sessionID := fmt.Sprintf("%s-%s", walletID, txID)

	// Check for duplicate
	sc.sessionsLock.RLock()
	_, isDuplicate := sc.activeSessions[sessionID]
	sc.sessionsLock.RUnlock()

	if isDuplicate {
		logger.Info("Duplicate signing request detected", "walletID", walletID, "txID", txID)
		return true
	}

	return false
}

func (sc *signConsumer) addSession(walletID, txID string) {
	sessionID := fmt.Sprintf("%s-%s", walletID, txID)
	sc.sessionsLock.Lock()
	sc.activeSessions[sessionID] = time.Now()
	sc.sessionsLock.Unlock()
}
