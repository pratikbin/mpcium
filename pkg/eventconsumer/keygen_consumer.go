package eventconsumer

import (
	"context"
	"errors"
	"time"

	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// Maximum time to wait for a keygen response.
	keygenResponseTimeout = 30 * time.Second
	// How often to poll for the reply message.
	keygenPollingInterval = 500 * time.Millisecond
)

// KeygenConsumer represents a consumer that processes keygen events.
type KeygenConsumer interface {
	// Run starts the consumer and blocks until the provided context is canceled.
	Run(ctx context.Context) error
	// Close performs a graceful shutdown of the consumer.
	Close() error
}

// keygenConsumer implements KeygenConsumer.
type keygenConsumer struct {
	natsConn           *nats.Conn
	pubsub             messaging.PubSub
	keygenRequestQueue messaging.MessageQueue

	// jsSub holds the JetStream subscription, so it can be cleaned up during Close().
	jsSub messaging.Subscription
}

// NewKeygenConsumer returns a new instance of KeygenConsumer.
func NewKeygenConsumer(natsConn *nats.Conn, keygenRequestQueue messaging.MessageQueue, pubsub messaging.PubSub) KeygenConsumer {
	return &keygenConsumer{
		natsConn:           natsConn,
		pubsub:             pubsub,
		keygenRequestQueue: keygenRequestQueue,
	}
}

// Run subscribes to keygen events and processes them until the context is canceled.
func (sc *keygenConsumer) Run(ctx context.Context) error {
	logger.Info("Starting key generation event consumer")

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping key generation event processing")
				return

			case <-ticker.C:
				logger.Info("Calling to fetch key generation events...")

				// No need for a separate fetch context since the fetch operation
				// is synchronous and completes before we'd cancel it
				err := sc.keygenRequestQueue.Fetch(2, func(msg jetstream.Msg) error {
					sc.handleKeygenEvent(msg)
					return nil
				})

				if err != nil {
					if !errors.Is(err, context.DeadlineExceeded) {
						logger.Error("Error fetching key generation events", err)
					}
				}
			}
		}
	}()

	return nil
}

func (sc *keygenConsumer) handleKeygenEvent(msg jetstream.Msg) {
	// Create a reply inbox to receive the keygen event response.
	replyInbox := nats.NewInbox()

	// Use a synchronous subscription for the reply inbox.
	replySub, err := sc.natsConn.SubscribeSync(replyInbox)
	if err != nil {
		logger.Error("KeygenConsumer: Failed to subscribe to reply inbox", err)
		_ = msg.Nak()
		return
	}
	defer func() {
		if err := replySub.Unsubscribe(); err != nil {
			logger.Warn("KeygenConsumer: Failed to unsubscribe from reply inbox", err)
		}
	}()

	// Publish the keygen event with the reply inbox.
	if err := sc.pubsub.PublishWithReply(MPCGenerateEvent, replyInbox, msg.Data()); err != nil {
		logger.Error("KeygenConsumer: Failed to publish keygen event with reply", err)
		_ = msg.Nak()
		return
	}

	// Poll for the reply message until timeout.
	deadline := time.Now().Add(keygenResponseTimeout)
	for time.Now().Before(deadline) {
		replyMsg, err := replySub.NextMsg(keygenPollingInterval)
		if err != nil {
			// If timeout occurs, continue trying.
			if err == nats.ErrTimeout {
				continue
			}
			logger.Error("KeygenConsumer: Error receiving reply message", err)
			break
		}
		if replyMsg != nil {
			logger.Info("KeygenConsumer: Completed keygen event reply received")
			if ackErr := msg.Ack(); ackErr != nil {
				logger.Error("KeygenConsumer: ACK failed", ackErr)
			}
			return
		}
	}

	logger.Warn("KeygenConsumer: Timeout waiting for keygen event response")
	_ = msg.Nak()
}

// Close unsubscribes from the JetStream subject and cleans up resources.
func (sc *keygenConsumer) Close() error {
	if sc.jsSub != nil {
		if err := sc.jsSub.Unsubscribe(); err != nil {
			logger.Error("KeygenConsumer: Failed to unsubscribe from JetStream", err)
			return err
		}
		logger.Info("KeygenConsumer: Unsubscribed from JetStream")
	}
	return nil
}
