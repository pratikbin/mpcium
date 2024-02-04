package messaging

import (
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
