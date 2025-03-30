package eventconsumer

import (
	"fmt"
	"time"

	"github.com/cryptoniumX/mpcium/pkg/event"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type signingConsumer struct {
	natsConn *nats.Conn
	pubsub   messaging.PubSub
	jsPubsub messaging.StreamPubsub
}

func NewSigningConsumer(natsConn *nats.Conn, jsPubsub messaging.StreamPubsub, pubsub messaging.PubSub) *signingConsumer {
	return &signingConsumer{
		natsConn: natsConn,
		pubsub:   pubsub,
		jsPubsub: jsPubsub,
	}
}

func (sc *signingConsumer) Run() {
	sub, err := sc.jsPubsub.Subscribe(event.SigningConsumerStream, event.SigningRequestEventTopic, func(message jetstream.Msg) {
		sc.handleSigningEvent(message)
	})

	if err != nil {
		panic(err)
	}
	defer sub.Unsubscribe()
}

func (sc *signingConsumer) handleSigningEvent(message jetstream.Msg) {
	replyInbox := nats.NewInbox()
	sub, err := sc.natsConn.SubscribeSync(replyInbox)
	if err != nil {
		logger.Error("SigingConsumer: Failed to subscribe to reply inbox", err)
		message.Nak()
		return
	}
	defer sub.Unsubscribe()

	sc.pubsub.PublishWithReply(event.MPCSigningEventTopic, replyInbox, message.Data())

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := sub.NextMsg(500 * time.Millisecond)
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			break
		}

		if msg != nil {
			fmt.Println("SigningConsumer: Completed signing event")
			message.Ack()
			return
		}
	}
	logger.Warn("SigningConsumer: Timeout waiting for signing event response")
	message.Nak()
}

func (sc *signingConsumer) Close() error {
	// Close the consumer and any resources it holds
	// For example, you can unsubscribe from the topic or close the connection
	return nil
}
