package eventconsumer

import (
	"context"
	"encoding/json"
	"log"
	"math/big"

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
		threshold := 2
		session, err := ec.node.CreateKeyGenSession(walletID, threshold, ec.genKeySucecssQueue)
		if err != nil {
			logger.Error("Failed to create key generation session", err)
			return
		}

		session.Init()
		ctx, done := context.WithCancel(context.Background())
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-session.ErrCh:
					logger.Error("Keygen session error", err)
				}
			}
		}()

		session.ListenToIncomingMessage()
		session.GenerateKey(done)
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

		logger.Info("Received signing event", "waleltID", msg.WalletID, "tx", msg.Tx)
		threshold := 2
		session, err := ec.node.CreateSigningSession(
			msg.WalletID,
			msg.TxID,
			msg.NetworkInternalCode,
			threshold,
			ec.signingSuccessQueue,
		)
		if err != nil {
			logger.Error("Failed to create signing session", err)
			return
		}

		txBigInt := new(big.Int).SetBytes(msg.Tx)
		session.Init(txBigInt)

		ctx, done := context.WithCancel(context.Background())
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-session.ErrCh:
					logger.Error("Signing session error", err)
				}
			}

		}()

		session.ListenToIncomingMessage()
		session.Sign(done)
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
