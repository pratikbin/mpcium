package messaging

import (
	"time"

	"github.com/nats-io/nats.go"
)

type DirectMessaging interface {
	Listen(target string, handler func(data []byte)) error
	Send(target string, data []byte) error
}

type natsDirectMessaging struct {
	natsConn *nats.Conn
}

func NewNatsDirectMessaging(natsConn *nats.Conn) DirectMessaging {
	return &natsDirectMessaging{
		natsConn: natsConn,
	}
}

func (d *natsDirectMessaging) Send(id string, message []byte) error {
	_, err := d.natsConn.Request(id, message, 1*time.Second)
	if err != nil {
		return err
	}
	return nil
}

func (d *natsDirectMessaging) Listen(id string, handler func(data []byte)) error {
	_, err := d.natsConn.Subscribe(id, func(m *nats.Msg) {
		handler(m.Data)
		m.Respond([]byte("OK"))
	})
	if err != nil {
		return err
	}
	return nil
}
