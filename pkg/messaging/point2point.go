package messaging

import (
	"fmt"
	"time"

	"github.com/fatih/color"
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
	color.Green("SEND DIRECT REQUEST", id)
	msg, err := d.natsConn.Request(id, message, 1*time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("msg = %+v\n", msg)
	return nil
}

func (d *natsDirectMessaging) Listen(id string, handler func(data []byte)) error {
	_, err := d.natsConn.Subscribe(id, func(m *nats.Msg) {
		color.Yellow("RECEIVE DIRECT REQUEST", id)
		handler(m.Data)
		m.Respond([]byte("OK"))
	})
	if err != nil {
		return err
	}
	return nil
}
