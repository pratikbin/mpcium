package eventconsumer

import (
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
}

type eventConsumer struct {
	node   *mpc.Node
	pubsub messaging.PubSub
}

func NewEventConsumer(node *mpc.Node, pubsub messaging.PubSub) EventConsumer {
	return &eventConsumer{
		node:   node,
		pubsub: pubsub,
	}
}

func (ec *eventConsumer) Run() {
	logger.Info("Event consumer is starting...!")
	err := ec.consumeKeyGenerationEvent()
	if err != nil {
		log.Fatal("Failed to consume key reconstruction event", err)
	}

	err = ec.consumeTxSigningEvent()
	if err != nil {
		log.Fatal("Failed to consume tx signing event", err)
	}
}

func (ec *eventConsumer) consumeKeyGenerationEvent() error {
	return ec.pubsub.Subscribe(MPCGenerateEvent, func(msg []byte) {
		walletID := string(msg)
		// TODO: threshold is configurable
		threshold := 2
		session, err := ec.node.CreateKeyGenSession(walletID, threshold)
		if err != nil {
			logger.Error("Failed to create key generation session", err)
			return
		}

		session.Init()
		go func() {
			for {
				select {
				case err := <-session.ErrCh:
					logger.Error("Keygen session error", err)
				}
			}

		}()

		go session.GenerateKey()
		// TODO -> done and close channel
		session.ListenToIncomingMessage()

	})
}

func (ec *eventConsumer) consumeTxSigningEvent() error {
	return ec.pubsub.Subscribe(MPCSignEvent, func(raw []byte) {
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
		)
		if err != nil {
			logger.Error("Failed to create signing session", err)
			return
		}

		txBigInt := new(big.Int).SetBytes(msg.Tx)
		session.Init(txBigInt)

		go func() {
			for {
				select {
				case err := <-session.ErrCh:
					logger.Error("Signing session error", err)
				}
			}

		}()

		go session.Sign()
		// TODO -> done and close channel
		session.ListenToIncomingMessage()
	})
}
