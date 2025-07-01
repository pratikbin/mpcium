package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

const (
	MPCGenerateEvent = "mpc:generate"
	MPCSignEvent     = "mpc:sign"
)

type EventConsumer interface {
	Run()
	Close() error
}

type eventConsumer struct {
	node         *mpc.Node
	pubsub       messaging.PubSub
	mpcThreshold int

	genKeySucecssQueue messaging.MessageQueue

	keyGenerationSub messaging.Subscription
	identityStore    identity.Store

	// Track active sessions with timestamps for cleanup
	activeSessions  map[string]time.Time // Maps "walletID-txID" to creation time
	sessionsLock    sync.RWMutex
	cleanupInterval time.Duration // How often to run cleanup
	sessionTimeout  time.Duration // How long before a session is considered stale
	cleanupStopChan chan struct{} // Signal to stop cleanup goroutine
}

func NewEventConsumer(
	node *mpc.Node,
	pubsub messaging.PubSub,
	genKeySucecssQueue messaging.MessageQueue,
	identityStore identity.Store,
) EventConsumer {
	ec := &eventConsumer{
		node:               node,
		pubsub:             pubsub,
		genKeySucecssQueue: genKeySucecssQueue,
		activeSessions:     make(map[string]time.Time),
		cleanupInterval:    5 * time.Minute,  // Run cleanup every 5 minutes
		sessionTimeout:     30 * time.Minute, // Consider sessions older than 30 minutes stale
		cleanupStopChan:    make(chan struct{}),
		mpcThreshold:       viper.GetInt("mpc_threshold"),
		identityStore:      identityStore,
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

	logger.Info("MPC Event consumer started...!")
}

func (ec *eventConsumer) consumeKeyGenerationEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCGenerateEvent, func(natMsg *nats.Msg) {
		raw := natMsg.Data
		var msg types.GenerateKeyMessage
		err := json.Unmarshal(raw, &msg)
		if err != nil {
			logger.Error("Failed to unmarshal signing message", err)
			return
		}
		logger.Info("Received key generation event", "msg", msg)

		err = ec.identityStore.VerifyInitiatorMessage(&msg)
		if err != nil {
			logger.Error("Failed to verify initiator message", err)
			return
		}

		walletID := msg.WalletID
		session, err := ec.node.CreateKeyGenSession(walletID, ec.mpcThreshold, ec.genKeySucecssQueue)
		if err != nil {
			logger.Error("Failed to create key generation session", err, "walletID", walletID)
			return
		}
		eddsaSession, err := ec.node.CreateEDDSAKeyGenSession(walletID, ec.mpcThreshold, ec.genKeySucecssQueue)
		if err != nil {
			logger.Error("Failed to create key generation session", err, "walletID", walletID)
			return
		}

		session.Init()
		eddsaSession.Init()

		ctx, done := context.WithCancel(context.Background())
		ctxEddsa, doneEddsa := context.WithCancel(context.Background())

		successEvent := &mpc.KeygenSuccessEvent{
			WalletID: walletID,
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			for {
				select {
				case <-ctx.Done():
					successEvent.ECDSAPubKey = session.GetPubKeyResult()
					wg.Done()
					return
				case err := <-session.ErrCh:
					logger.Error("Keygen session error", err)
				}
			}
		}()

		go func() {
			for {
				select {
				case <-ctxEddsa.Done():
					successEvent.EDDSAPubKey = eddsaSession.GetPubKeyResult()
					wg.Done()
					return
				case err := <-eddsaSession.ErrCh:
					logger.Error("Keygen session error", err)
				}
			}
		}()

		session.ListenToIncomingMessageAsync()
		eddsaSession.ListenToIncomingMessageAsync()
		// TODO: replace sleep with distributed lock
		time.Sleep(1 * time.Second)

		go session.GenerateKey(done)
		go eddsaSession.GenerateKey(doneEddsa)

		wg.Wait()
		logger.Info("Closing session successfully!", "event", successEvent)

		successEventBytes, err := json.Marshal(successEvent)
		if err != nil {
			logger.Error("Failed to marshal keygen success event", err)
			return
		}

		err = ec.genKeySucecssQueue.Enqueue(fmt.Sprintf(mpc.TypeGenerateWalletSuccess, walletID), successEventBytes, &messaging.EnqueueOptions{
			IdempotententKey: fmt.Sprintf(mpc.TypeGenerateWalletSuccess, walletID),
		})
		if err != nil {
			logger.Error("Failed to publish key generation success message", err)
			return
		}

		logger.Info("[COMPLETED KEY GEN] Key generation completed successfully", "walletID", walletID)

	})

	ec.keyGenerationSub = sub
	if err != nil {
		return err
	}
	return nil
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

// Close and clean up
func (ec *eventConsumer) Close() error {
	// Signal cleanup routine to stop
	close(ec.cleanupStopChan)

	err := ec.keyGenerationSub.Unsubscribe()
	if err != nil {
		return err
	}

	return nil
}
