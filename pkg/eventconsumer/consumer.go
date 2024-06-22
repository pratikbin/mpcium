package eventconsumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/cryptoniumX/mpcium/pkg/mpc"
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
	node   *mpc.Node
	pubsub messaging.PubSub

	genKeySucecssQueue  messaging.MessageQueue
	signingSuccessQueue messaging.MessageQueue

	keyGenerationSub messaging.Subscription
	signingSub       messaging.Subscription
}

func NewEventConsumer(
	node *mpc.Node,
	pubsub messaging.PubSub,
	genKeySucecssQueue messaging.MessageQueue,
	signingSuccessQueue messaging.MessageQueue,
) EventConsumer {
	return &eventConsumer{
		node:                node,
		pubsub:              pubsub,
		genKeySucecssQueue:  genKeySucecssQueue,
		signingSuccessQueue: signingSuccessQueue,
	}
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

	logger.Info("MPC Event consumer started...!")
}

func (ec *eventConsumer) consumeKeyGenerationEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCGenerateEvent, func(msg []byte) {
		walletID := string(msg)
		// TODO: threshold is configurable
		threshold := 1
		session, err := ec.node.CreateKeyGenSession(walletID, threshold, ec.genKeySucecssQueue)
		if err != nil {
			logger.Error("Failed to create key generation session", err, "walletID", walletID)
			return
		}
		eddsaSession, err := ec.node.CreateEDDSAKeyGenSession(walletID, threshold, ec.genKeySucecssQueue)
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
					successEvent.S256PubKey = session.GetPubKeyResult()
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
		if err != nil {
			logger.Error("Errors when closing sessions", err)
		}
		logger.Info("Closing section successfully!", "event", successEvent)

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
		if err != nil {
			logger.Error("Failed to close session", err)
		}

	})

	ec.keyGenerationSub = sub
	if err != nil {
		return err
	}
	return nil
}

func (ec *eventConsumer) consumeTxSigningEvent() error {
	sub, err := ec.pubsub.Subscribe(MPCSignEvent, func(raw []byte) {
		var msg SignTxMessage
		err := json.Unmarshal(raw, &msg)
		if err != nil {
			logger.Error("Failed to unmarshal message", err)
			return
		}

		logger.Info("Received signing event", "waleltID", msg.WalletID, "type", msg.KeyType, "tx", msg.Tx)
		threshold := 1

		var session mpc.ISigningSession
		switch msg.KeyType {
		case KeyTypeSecp256k1:
			session, err = ec.node.CreateSigningSession(
				msg.WalletID,
				msg.TxID,
				msg.NetworkInternalCode,
				threshold,
				ec.signingSuccessQueue,
			)
		case KeyTypeEd25519:
			session, err = ec.node.CreateEDDSASigningSession(
				msg.WalletID,
				msg.TxID,
				msg.NetworkInternalCode,
				threshold,
				ec.signingSuccessQueue,
			)

		}

		if err != nil {
			logger.Error("Failed to create signing session", err)
			return
		}

		txBigInt := new(big.Int).SetBytes(msg.Tx)
		err = session.Init(txBigInt)
		if err != nil {
			logger.Error("Failed to init signing session, terminate session", err, "walletID", msg.WalletID)
			return
		}

		ctx, done := context.WithCancel(context.Background())
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-session.ErrChan():
					logger.Error("Signing session error", err)
				}
			}
		}()

		session.ListenToIncomingMessageAsync()
		// TODO: use consul distributed lock here
		time.Sleep(1 * time.Second)
		go session.Sign(done) // use go routine to not block the event susbscriber
	})

	ec.signingSub = sub
	if err != nil {
		return err
	}

	return nil
}

// Close and clean up
func (ec *eventConsumer) Close() error {
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
