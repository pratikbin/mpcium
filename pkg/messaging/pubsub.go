package messaging

import (
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/nats-io/nats.go"
)

type PubSub interface {
	Publish(topic string, message []byte) error
	Subscribe(topic string, handler func(message []byte)) error
}

type natsPubSub struct {
	natsConn *nats.Conn
}

func NewNATSPubSub(natsConn *nats.Conn) PubSub {
	return &natsPubSub{natsConn}
}

func (n *natsPubSub) Publish(topic string, message []byte) error {
	logger.Infof("Publishing to topic %s", topic)
	return n.natsConn.Publish(topic, message)
}

func (n *natsPubSub) Subscribe(topic string, handler func(message []byte)) error {
	// TODO: Handle subscription
	// handle more fields in msg
	_, err := n.natsConn.Subscribe(topic, func(msg *nats.Msg) {
		handler(msg.Data)
	})

	return err
}
