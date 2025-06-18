package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/event"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/node"
	"github.com/fystack/mpcium/pkg/mpc/session"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

const (
	MPCGenerateEvent  = "mpc:generate"
	MPCSignEvent      = "mpc:sign"
	MPCResharingEvent = "mpc:reshare"

	// Default version for keygen
	DefaultVersion int = 1
)

type EventConsumer interface {
	Run()
	Close() error
}

type eventConsumer struct {
	node         *node.Node
	pubsub       messaging.PubSub
	mpcThreshold int

	genKeySucecssQueue   messaging.MessageQueue
	signingResultQueue   messaging.MessageQueue
	resharingResultQueue messaging.MessageQueue

	keyGenerationSub messaging.Subscription
	signingSub       messaging.Subscription
	resharingSub     messaging.Subscription
	identityStore    identity.Store

	// Track active sessions with timestamps for cleanup
	activeSessions  map[string]time.Time // Maps "walletID-txID" to creation time
	sessionsLock    sync.RWMutex
	cleanupInterval time.Duration // How often to run cleanup
	sessionTimeout  time.Duration // How long before a session is considered stale
	cleanupStopChan chan struct{} // Signal to stop cleanup goroutine
}

func NewEventConsumer(
	node *node.Node,
	pubsub messaging.PubSub,
	genKeySucecssQueue messaging.MessageQueue,
	signingResultQueue messaging.MessageQueue,
	resharingResultQueue messaging.MessageQueue,
	identityStore identity.Store,
) EventConsumer {
	ec := &eventConsumer{
		node:                 node,
		pubsub:               pubsub,
		genKeySucecssQueue:   genKeySucecssQueue,
		signingResultQueue:   signingResultQueue,
		resharingResultQueue: resharingResultQueue,
		activeSessions:       make(map[string]time.Time),
		cleanupInterval:      5 * time.Minute,  // Run cleanup every 5 minutes
		sessionTimeout:       30 * time.Minute, // Consider sessions older than 30 minutes stale
		cleanupStopChan:      make(chan struct{}),
		mpcThreshold:         viper.GetInt("mpc_threshold"),
		identityStore:        identityStore,
	}

	// Start background cleanup goroutine
	go ec.sessionCleanupRoutine()

	return ec
}

func (ec *eventConsumer) Run() {
	err := ec.consumeKeyGenerationEvent()
	if err != nil {
		log.Fatal("Failed to consume key reconstruction event", err)
	}

	err = ec.consumeTxSigningEvent()
	if err != nil {
		log.Fatal("Failed to consume tx signing event", err)
	}

	err = ec.consumeResharingEvent()
	if err != nil {
		log.Fatal("Failed to consume resharing event", err)
	}

	logger.Info("MPC Event consumer started...!")
}
func (ec *eventConsumer) consumeKeyGenerationEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCGenerateEvent, func(natMsg *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := ec.handleKeyGenerationEvent(ctx, natMsg.Data); err != nil {
			logger.Error("Failed to handle key generation event", err)
		}
	})

	if err != nil {
		return err
	}

	ec.keyGenerationSub = sub
	return nil
}

func (ec *eventConsumer) handleKeyGenerationEvent(ctx context.Context, raw []byte) error {
	var msg types.GenerateKeyMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}
	logger.Info("Received key generation event", "msg", msg)

	if err := ec.identityStore.VerifyInitiatorMessage(&msg); err != nil {
		return fmt.Errorf("verify initiator: %w", err)
	}

	walletID := msg.WalletID
	successEvent := &event.KeygenSuccessEvent{WalletID: walletID}
	var wg sync.WaitGroup

	// Start ECDSA and EDDSA sessions
	for _, keyType := range []types.KeyType{types.KeyTypeSecp256k1, types.KeyTypeEd25519} {
		kgSession, err := ec.node.CreateKeygenSession(keyType, walletID, ec.mpcThreshold, ec.genKeySucecssQueue)
		if err != nil {
			return fmt.Errorf("create %v session: %w", keyType, err)
		}

		defer kgSession.Close()

		kgSession.Listen()
		time.Sleep(1 * time.Second)
		wg.Add(1)

		go func(s session.Session, kt types.KeyType) {
			defer wg.Done()

			s.StartKeygen(ctx, s.Send, func(data []byte) {
				err := s.SaveKey(ec.node.GetReadyPeersIncludeSelf(), ec.mpcThreshold, DefaultVersion, data)
				if err != nil {
					logger.Error("Failed to save key", err)
				}
				logger.Info("Saved key", "type", kt, "walletID", walletID, "threshold", ec.mpcThreshold, "version", DefaultVersion, "data", len(data))
				if pubKey, err := s.GetPublicKey(data); err == nil {
					switch kt {
					case types.KeyTypeSecp256k1:
						successEvent.ECDSAPubKey = pubKey
					case types.KeyTypeEd25519:
						successEvent.EDDSAPubKey = pubKey
					}
				}
			})

			if err != nil {
				logger.Error("Keygen failed", err, "keyType", kt)
			}
		}(kgSession, keyType)
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		logger.Warn("Keygen timed out", "walletID", walletID)
		return ctx.Err()
	case <-doneCh:
		// All keygens done
	}

	successBytes, err := json.Marshal(successEvent)
	if err != nil {
		return fmt.Errorf("marshal keygen error: %w", err)
	}

	err = ec.genKeySucecssQueue.Enqueue(event.KeygenSuccessEventTopic, successBytes, &messaging.EnqueueOptions{
		IdempotententKey: fmt.Sprintf(event.TypeGenerateWalletSuccess, walletID),
	})
	if err != nil {
		return fmt.Errorf("enqueue keygen error: %w", err)
	}

	logger.Info("[COMPLETED KEY GEN] Key generation completed successfully", "walletID", walletID)
	return nil
}

func (ec *eventConsumer) consumeTxSigningEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCSignEvent, func(natMsg *nats.Msg) {
		raw := natMsg.Data
		var msg types.SignTxMessage
		err := json.Unmarshal(raw, &msg)
		if err != nil {
			logger.Error("Failed to unmarshal signing message", err)
			return
		}

		err = ec.identityStore.VerifyInitiatorMessage(&msg)
		if err != nil {
			logger.Error("Failed to verify initiator message", err)
			return
		}

		logger.Info("Received signing event", "msg", msg)

		// Check for duplicate session and track if new
		if ec.checkDuplicateSession(msg.WalletID, msg.TxID) {
			natMsg.Term()
			return
		}

		// Add session to tracking before starting
		ec.addSession(msg.WalletID, msg.TxID)

		keyInfoVersion, err := ec.node.GetKeyInfoVersion(msg.KeyType, msg.WalletID)
		if err != nil {
			logger.Error("Failed to get party version", err)
			ec.removeSession(msg.WalletID, msg.TxID)
			return
		}

		signingSession, err := ec.node.CreateSigningSession(
			msg.KeyType,
			msg.WalletID,
			msg.TxID,
			keyInfoVersion,
			ec.mpcThreshold,
			ec.signingResultQueue,
		)

		if err != nil {
			ec.handleSigningSessionError(
				msg.WalletID,
				msg.TxID,
				msg.NetworkInternalCode,
				err,
				"Failed to create signing session",
				natMsg,
			)
			ec.removeSession(msg.WalletID, msg.TxID)
			return
		}

		go signingSession.Listen()

		txBigInt := new(big.Int).SetBytes(msg.Tx)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			signingSession.StartSigning(ctx, txBigInt, signingSession.Send, func(data []byte) {
				cancel()
				signatureData, err := signingSession.VerifySignature(msg.Tx, data)
				if err != nil {
					logger.Error("Failed to verify signature", err)
					ec.removeSession(msg.WalletID, msg.TxID)
					return
				}

				signingResult := event.SigningResultEvent{
					WalletID:            msg.WalletID,
					TxID:                msg.TxID,
					NetworkInternalCode: msg.NetworkInternalCode,
					ResultType:          event.SigningResultTypeSuccess,
					Signature:           data,
					R:                   signatureData.R,
					S:                   signatureData.S,
					SignatureRecovery:   signatureData.SignatureRecovery,
				}

				signingResultBytes, err := json.Marshal(signingResult)
				if err != nil {
					logger.Error("Failed to marshal signing result event", err)
					ec.removeSession(msg.WalletID, msg.TxID)
					return
				}

				err = ec.signingResultQueue.Enqueue(event.SigningResultCompleteTopic, signingResultBytes, &messaging.EnqueueOptions{
					IdempotententKey: fmt.Sprintf(event.TypeSigningResultComplete, msg.WalletID, msg.TxID),
				})
				if err != nil {
					logger.Error("Failed to publish signing result event", err)
					ec.removeSession(msg.WalletID, msg.TxID)
					return
				}

				logger.Info("Signing completed", "walletID", msg.WalletID, "txID", msg.TxID, "data", len(data))
				ec.removeSession(msg.WalletID, msg.TxID)
			})
		}()

		go func() {
			for err := range signingSession.ErrCh() {
				logger.Error("Error from session", err)
				ec.handleSigningSessionError(
					msg.WalletID,
					msg.TxID,
					msg.NetworkInternalCode,
					err,
					"Failed to sign tx",
					natMsg,
				)
				ec.removeSession(msg.WalletID, msg.TxID)
			}
		}()
	})

	ec.signingSub = sub
	if err != nil {
		return err
	}

	return nil
}

func (ec *eventConsumer) consumeResharingEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCResharingEvent, func(natMsg *nats.Msg) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := ec.handleReshareEvent(ctx, natMsg.Data); err != nil {
			logger.Error("Failed to handle resharing event", err)
		}
	})
	if err != nil {
		return err
	}

	ec.resharingSub = sub
	return nil
}

func (ec *eventConsumer) handleReshareEvent(ctx context.Context, raw []byte) error {
	var msg types.ResharingMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("unmarshal message: %w", err)
	}
	logger.Info("Received resharing event",
		"walletID", msg.WalletID,
		"oldThreshold", ec.mpcThreshold,
		"newThreshold", msg.NewThreshold)

	if err := ec.identityStore.VerifyInitiatorMessage(&msg); err != nil {
		return fmt.Errorf("verify initiator: %w", err)
	}

	keyInfoVersion, _ := ec.node.GetKeyInfoVersion(msg.KeyType, msg.WalletID)

	oldSession, err := ec.node.CreateResharingSession(true, msg.KeyType, msg.WalletID, ec.mpcThreshold, keyInfoVersion, ec.resharingResultQueue)
	if err != nil {
		return fmt.Errorf("create old session: %w", err)
	}

	newSession, err := ec.node.CreateResharingSession(false, msg.KeyType, msg.WalletID, msg.NewThreshold, keyInfoVersion, ec.resharingResultQueue)
	if err != nil {
		return fmt.Errorf("create new session: %w", err)
	}

	go oldSession.Listen()
	go newSession.Listen()

	successEvent := &event.ResharingSuccessEvent{WalletID: msg.WalletID}

	var wg sync.WaitGroup
	wg.Add(2)

	// Error monitor
	go func() {
		for {
			select {
			case err := <-oldSession.ErrCh():
				logger.Error("Error from old session", err)
			case err := <-newSession.ErrCh():
				logger.Error("Error from new session", err)
			}
		}
	}()

	// Start old session
	go func() {
		ctxOld, cancelOld := context.WithCancel(ctx)
		defer cancelOld()
		oldSession.StartResharing(ctxOld,
			oldSession.PartyIDs(),
			newSession.PartyIDs(),
			ec.mpcThreshold,
			msg.NewThreshold,
			oldSession.Send,
			func([]byte) { wg.Done() },
		)
	}()

	// Start new session
	go func() {
		ctxNew, cancelNew := context.WithCancel(ctx)
		defer cancelNew()
		newSession.StartResharing(ctxNew,
			oldSession.PartyIDs(),
			newSession.PartyIDs(),
			ec.mpcThreshold,
			msg.NewThreshold,
			newSession.Send,
			func(data []byte) {
				if pubKey, err := newSession.GetPublicKey(data); err == nil {
					newSession.SaveKey(ec.node.GetReadyPeersIncludeSelf(), msg.NewThreshold, keyInfoVersion+1, data)
					if msg.KeyType == types.KeyTypeSecp256k1 {
						successEvent.ECDSAPubKey = pubKey
					} else {
						successEvent.EDDSAPubKey = pubKey
					}
				} else {
					logger.Error("Failed to get public key", err)
				}
				wg.Done()
			},
		)
	}()

	wg.Wait()

	eventBytes, err := json.Marshal(successEvent)
	if err != nil {
		return fmt.Errorf("marshal success event: %w", err)
	}

	err = ec.resharingResultQueue.Enqueue(
		event.ResharingSuccessEventTopic,
		eventBytes,
		&messaging.EnqueueOptions{
			IdempotententKey: fmt.Sprintf(event.TypeResharingSuccess, msg.WalletID, keyInfoVersion+1),
		},
	)
	if err != nil {
		return fmt.Errorf("enqueue resharing success: %w", err)
	}

	logger.Info("[COMPLETED RESH] Resharing completed successfully", "walletID", msg.WalletID)
	return nil
}

func (ec *eventConsumer) handleSigningSessionError(walletID, txID, NetworkInternalCode string, err error, errMsg string, natMsg *nats.Msg) {
	logger.Error("signing session error", err, "walletID", walletID, "txID", txID, "error", errMsg)
	signingResult := event.SigningResultEvent{
		ResultType:          event.SigningResultTypeError,
		NetworkInternalCode: NetworkInternalCode,
		WalletID:            walletID,
		TxID:                txID,
		ErrorReason:         errMsg,
	}

	signingResultBytes, err := json.Marshal(signingResult)
	if err != nil {
		logger.Error("failed to marshal signing result event", err)
		return
	}

	natMsg.Ack()
	err = ec.signingResultQueue.Enqueue(event.SigningResultCompleteTopic, signingResultBytes, &messaging.EnqueueOptions{
		IdempotententKey: txID,
	})
	if err != nil {
		logger.Error("Failed to publish signing result event", err)
		return
	}
}

// Add a cleanup routine that runs periodically
func (ec *eventConsumer) sessionCleanupRoutine() {
	ticker := time.NewTicker(ec.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ec.cleanupStaleSessions()
		case <-ec.cleanupStopChan:
			return
		}
	}
}

// Cleanup stale sessions
func (ec *eventConsumer) cleanupStaleSessions() {
	now := time.Now()
	ec.sessionsLock.Lock()
	defer ec.sessionsLock.Unlock()

	for sessionID, creationTime := range ec.activeSessions {
		if now.Sub(creationTime) > ec.sessionTimeout {
			logger.Info("Cleaning up stale session", "sessionID", sessionID, "age", now.Sub(creationTime))
			delete(ec.activeSessions, sessionID)
		}
	}
}

// markSessionAsActive marks a session as active with the current timestamp
func (ec *eventConsumer) addSession(walletID, txID string) {
	sessionID := fmt.Sprintf("%s-%s", walletID, txID)
	ec.sessionsLock.Lock()
	ec.activeSessions[sessionID] = time.Now()
	ec.sessionsLock.Unlock()
}

// Remove a session from tracking
func (ec *eventConsumer) removeSession(walletID, txID string) {
	sessionID := fmt.Sprintf("%s-%s", walletID, txID)
	ec.sessionsLock.Lock()
	delete(ec.activeSessions, sessionID)
	ec.sessionsLock.Unlock()
}

// checkAndTrackSession checks if a session already exists and tracks it if new.
// Returns true if the session is a duplicate.
func (ec *eventConsumer) checkDuplicateSession(walletID, txID string) bool {
	sessionID := fmt.Sprintf("%s-%s", walletID, txID)

	// Check for duplicate
	ec.sessionsLock.RLock()
	_, isDuplicate := ec.activeSessions[sessionID]
	ec.sessionsLock.RUnlock()

	if isDuplicate {
		logger.Info("Duplicate signing request detected", "walletID", walletID, "txID", txID)
		return true
	}

	return false
}

// Close and clean up
func (ec *eventConsumer) Close() error {
	// Signal cleanup routine to stop
	close(ec.cleanupStopChan)

	err := ec.keyGenerationSub.Unsubscribe()
	if err != nil {
		return err
	}
	err = ec.signingSub.Unsubscribe()
	if err != nil {
		return err
	}

	return nil
}
