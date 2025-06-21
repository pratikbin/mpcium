package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/common/concurrency"
	"github.com/fystack/mpcium/pkg/event"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/monitoring"
	"github.com/fystack/mpcium/pkg/mpc/node"
	"github.com/fystack/mpcium/pkg/mpc/session"
	"github.com/fystack/mpcium/pkg/tsslimiter"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	MPCGenerateEvent  = "mpc:generate"
	MPCSignEvent      = "mpc:sign"
	MPCResharingEvent = "mpc:reshare"

	// Default version for keygen
	DefaultVersion        int = 1
	SessionTimeout            = 1 * time.Minute
	MaxConcurrentSessions     = 5
	// how long the entire handler will wait for *all* sessions + publishing:
	HandlerTimeout = 2 * time.Minute
)

type EventConsumer interface {
	Run()
	Close() error
}

func Elaps(start time.Time, text string) {
	elapsed := time.Since(start)
	fmt.Printf("%s, Elapsed time: %d ms\n", text, elapsed.Milliseconds())
}

type eventConsumer struct {
	node         *node.Node
	pubsub       messaging.PubSub
	mpcThreshold int

	genKeySuccessQueue   messaging.MessageQueue
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
	limiterQueue    tsslimiter.Queue
	tracer          trace.Tracer
}

func NewEventConsumer(
	node *node.Node,
	pubsub messaging.PubSub,
	genKeySuccessQueue messaging.MessageQueue,
	signingResultQueue messaging.MessageQueue,
	resharingResultQueue messaging.MessageQueue,
	identityStore identity.Store,
) EventConsumer {
	limiter := tsslimiter.NewWeightedLimiter(concurrency.GetTSSConcurrencyLimit())
	limiterQueue := tsslimiter.NewWeightedQueue(limiter, 100)

	ec := &eventConsumer{
		node:                 node,
		pubsub:               pubsub,
		genKeySuccessQueue:   genKeySuccessQueue,
		signingResultQueue:   signingResultQueue,
		resharingResultQueue: resharingResultQueue,
		activeSessions:       make(map[string]time.Time),
		cleanupInterval:      5 * time.Minute,  // Run cleanup every 5 minutes
		sessionTimeout:       30 * time.Minute, // Consider sessions older than 30 minutes stale
		cleanupStopChan:      make(chan struct{}),
		mpcThreshold:         viper.GetInt("mpc_threshold"),
		identityStore:        identityStore,
		limiterQueue:         limiterQueue,
		tracer:               otel.Tracer("mpcium/eventconsumer"),
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
	// Create session limiter channel with capacity 5
	sub, err := ec.pubsub.Subscribe(MPCGenerateEvent, func(natMsg *nats.Msg) {
		carrier := messaging.NewNatsHeaderCarrier(natMsg.Header)
		parentCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

		ctx, span := ec.tracer.Start(parentCtx, "consumeKeyGenerationEvent")
		defer span.End()

		logger.Info("Received key generation event, enqueuing combined job", "subject", natMsg.Subject)

		job := tsslimiter.SessionJob{
			Type: tsslimiter.SessionKeygenCombined,
			Run: func() error {
				return ec.handleKeyGenerationEvent(ctx, natMsg.Data)
			},
			OnError: func(err error) {
				logger.Error("Failed to handle key generation event", err, "message", natMsg.Data)
			},
			Name: string(natMsg.Data),
		}
		ec.limiterQueue.Enqueue(job)
	})

	if err != nil {
		return err
	}

	ec.keyGenerationSub = sub
	return nil
}

func (ec *eventConsumer) handleKeyGenerationEvent(parentCtx context.Context, raw []byte) error {
	handlerCtx, handlerCancel := context.WithTimeout(parentCtx, HandlerTimeout)
	defer handlerCancel()

	ctx, span := ec.tracer.Start(handlerCtx, "handleKeyGenerationEvent")

	defer span.End()

	// 1) decode and verify
	var msg types.GenerateKeyMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("unmarshal message: %w", err)
	}
	if err := ec.identityStore.VerifyInitiatorMessage(&msg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("verify initiator: %w", err)
	}

	walletID := msg.WalletID
	span.SetAttributes(attribute.String("wallet_id", walletID))
	logger.Info("Processing combined key generation job", "walletID", walletID)
	successEvent := &event.KeygenSuccessEvent{WalletID: walletID}

	// 2) prepare both sessions
	s0, err := ec.node.CreateKeygenSession(types.KeyTypeSecp256k1, walletID, ec.mpcThreshold, ec.genKeySuccessQueue)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("create ECDSA session: %w", err)
	}
	defer s0.Close()

	s1, err := ec.node.CreateKeygenSession(types.KeyTypeEd25519, walletID, ec.mpcThreshold, ec.genKeySuccessQueue)
	if err != nil {
		s0.Close()
		span.RecordError(err)
		return fmt.Errorf("create EDDSA session: %w", err)
	}
	defer s1.Close()

	s0.Listen()
	s1.Listen()

	// 3) Start both key generation routines and wait for them to complete
	var wg sync.WaitGroup
	wg.Add(2)

	runKeygen := func(s session.Session, keyType types.KeyType) {
		sessionCtx, sessionCancel := context.WithTimeout(ctx, SessionTimeout)
		defer sessionCancel()

		s.StartKeygen(sessionCtx, s.Send, func(data []byte) {
			defer wg.Done()
			logger.Info("[callback] StartKeygen fired", "walletID", walletID, "keyType", keyType)
			if err := s.SaveKey(ec.node.GetReadyPeersIncludeSelf(), ec.mpcThreshold, DefaultVersion, data); err != nil {
				logger.Error("Failed to save key", err, "walletID", walletID, "keyType", keyType)
			}

			if pubKey, err := s.GetPublicKey(data); err == nil {
				switch keyType {
				case types.KeyTypeSecp256k1:
					successEvent.ECDSAPubKey = pubKey
				case types.KeyTypeEd25519:
					successEvent.EDDSAPubKey = pubKey
				}
			} else {
				logger.Error("Failed to get public key", err, "walletID", walletID, "keyType", keyType)
			}
		})
	}

	go runKeygen(s0, types.KeyTypeSecp256k1)
	go runKeygen(s1, types.KeyTypeEd25519)

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-handlerCtx.Done():
		logger.Warn("Keygen timed out while waiting for MPC sessions to complete", "walletID", walletID, "error", handlerCtx.Err())
		span.RecordError(handlerCtx.Err())
		return handlerCtx.Err()
	case <-doneCh:
		logger.Info("Both key generation sessions completed.", "walletID", walletID)
	}

	// 4) marshal & publish success
	successBytes, err := json.Marshal(successEvent)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("marshal success event: %w", err)
	}
	if err := ec.genKeySuccessQueue.Enqueue(
		fmt.Sprintf(event.TypeGenerateWalletSuccess, walletID),
		successBytes,
		&messaging.EnqueueOptions{
			IdempotententKey: fmt.Sprintf(event.TypeGenerateWalletSuccess, walletID),
		},
	); err != nil {
		span.RecordError(err)
		return fmt.Errorf("enqueue success event: %w", err)
	}

	logger.Info("[COMPLETED KEY GEN] Key generation completed successfully", "walletID", walletID)
	return nil
}

func (ec *eventConsumer) consumeTxSigningEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCSignEvent, func(natMsg *nats.Msg) {
		carrier := messaging.NewNatsHeaderCarrier(natMsg.Header)
		parentCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

		ctx, span := ec.tracer.Start(parentCtx, "consumeTxSigningEvent")
		defer span.End()

		raw := natMsg.Data
		var msg types.SignTxMessage
		err := json.Unmarshal(raw, &msg)
		if err != nil {
			logger.Error("Failed to unmarshal signing message", err)
			span.RecordError(err)
			return
		}

		span.SetAttributes(
			attribute.String("wallet_id", msg.WalletID),
			attribute.String("tx_id", msg.TxID),
		)

		err = ec.identityStore.VerifyInitiatorMessage(&msg)
		if err != nil {
			logger.Error("Failed to verify initiator message", err)
			span.RecordError(err)
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
			span.RecordError(err)
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
				ctx,
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
			sessionCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			signingSession.StartSigning(sessionCtx, txBigInt, signingSession.Send, func(data []byte) {
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
					ctx,
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
		carrier := messaging.NewNatsHeaderCarrier(natMsg.Header)
		parentCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

		ctx, span := ec.tracer.Start(parentCtx, "consumeResharingEvent")
		defer span.End()

		reshareCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		if err := ec.handleReshareEvent(reshareCtx, natMsg.Data); err != nil {
			span.RecordError(err)
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
	ctx, span := ec.tracer.Start(ctx, "handleReshareEvent")
	defer span.End()

	var msg types.ResharingMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("unmarshal message: %w", err)
	}
	logger.Info("Received resharing event",
		"walletID", msg.WalletID,
		"oldThreshold", ec.mpcThreshold,
		"newThreshold", msg.NewThreshold)
	span.SetAttributes(attribute.String("wallet_id", msg.WalletID))

	if err := ec.identityStore.VerifyInitiatorMessage(&msg); err != nil {
		span.RecordError(err)
		return fmt.Errorf("verify initiator: %w", err)
	}

	keyInfoVersion, _ := ec.node.GetKeyInfoVersion(msg.KeyType, msg.WalletID)

	oldSession, err := ec.node.CreateResharingSession(true, msg.KeyType, msg.WalletID, ec.mpcThreshold, keyInfoVersion, ec.resharingResultQueue)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("create old session: %w", err)
	}

	newSession, err := ec.node.CreateResharingSession(false, msg.KeyType, msg.WalletID, msg.NewThreshold, keyInfoVersion, ec.resharingResultQueue)
	if err != nil {
		span.RecordError(err)
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
		span.RecordError(err)
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
		span.RecordError(err)
		return fmt.Errorf("enqueue resharing success: %w", err)
	}

	logger.Info("[COMPLETED RESH] Resharing completed successfully", "walletID", msg.WalletID)
	return nil
}

func (ec *eventConsumer) handleSigningSessionError(ctx context.Context, walletID, txID, NetworkInternalCode string, err error, errMsg string, natMsg *nats.Msg) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetAttributes(
		attribute.String("error.message", errMsg),
	)

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

	if ec.keyGenerationSub != nil {
		if err := ec.keyGenerationSub.Unsubscribe(); err != nil {
			return err
		}
	}
	if ec.signingSub != nil {
		if err := ec.signingSub.Unsubscribe(); err != nil {
			return err
		}
	}
	if ec.resharingSub != nil {
		if err := ec.resharingSub.Unsubscribe(); err != nil {
			return err
		}
	}

	// Ensure all monitoring logs are written to disk before exiting.
	monitoring.Close()

	return nil
}
