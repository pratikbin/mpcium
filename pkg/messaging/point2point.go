package messaging

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

type DirectMessaging interface {
	Listen(target string, handler func(data []byte)) error
	Send(target string, data []byte) error
}

type natsDirectMessaging struct {
	natsConn *nats.Conn
	ch       chan []byte
}

func NewNatsDirectMessaging(natsConn *nats.Conn) DirectMessaging {
	return &natsDirectMessaging{
		natsConn: natsConn,
		ch:       make(chan []byte),
	}
}

func (d *natsDirectMessaging) Send(id string, message []byte) error {
	fmt.Println("SEND DIRECT REQUEST", id)
	msg, err := d.natsConn.Request(id, message, time.Second)
	if err != nil {
		return err
	}
	fmt.Printf("msg = %+v\n", msg)
	return nil
}

func (d *natsDirectMessaging) Listen(id string, handler func(data []byte)) error {
	_, err := d.natsConn.Subscribe(id, func(m *nats.Msg) {
		d.ch <- m.Data
		handler(m.Data)
		m.Respond([]byte("OK"))
	})
	if err != nil {
		return err
	}
	return nil
}
