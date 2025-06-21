package messaging

import (
	"context"

	"github.com/fystack/mpcium/pkg/logger"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
)

type DirectMessaging interface {
	Send(ctx context.Context, topic string, data []byte) error
	Listen(topic string, onReceive func(msg *nats.Msg)) (Subscription, error)
	Close()
}

type natsDirectMessaging struct {
	conn *nats.Conn
}

func NewNatsDirectMessaging(conn *nats.Conn) DirectMessaging {
	return &natsDirectMessaging{conn}
}

func (d *natsDirectMessaging) Send(ctx context.Context, topic string, data []byte) error {
	logger.Debug("[NATS] Sending direct message", "topic", topic)
	msg := &nats.Msg{
		Subject: topic,
		Data:    data,
		Header:  nats.Header{},
	}
	otel.GetTextMapPropagator().Inject(ctx, NewNatsHeaderCarrier(msg.Header))
	return d.conn.PublishMsg(msg)
}

func (d *natsDirectMessaging) Listen(topic string, onReceive func(msg *nats.Msg)) (Subscription, error) {
	sub, err := d.conn.Subscribe(topic, func(msg *nats.Msg) {
		onReceive(msg)
	})
	if err != nil {
		return nil, err
	}
	return &natsSubscription{subscription: sub}, nil
}

func (d *natsDirectMessaging) Close() {
	d.conn.Close()
}
