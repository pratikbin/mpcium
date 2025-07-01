package messaging

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/fystack/mpcium/pkg/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Subscription interface {
	Unsubscribe() error
}

type PubSub interface {
	Publish(topic string, message []byte) error
	PublishWithReply(ttopic, reply string, data []byte) error
	Subscribe(topic string, handler func(msg *nats.Msg)) (Subscription, error)
}

type natsPubSub struct {
	natsConn *nats.Conn
}

type natsSubscription struct {
	subscription *nats.Subscription
}

type jetstreamSubscription struct {
	consumer jetstream.Consumer
}

func (ns *natsSubscription) Unsubscribe() error {
	return ns.subscription.Unsubscribe()
}

func (js *jetstreamSubscription) Unsubscribe() error {
	return nil
}

func NewNATSPubSub(natsConn *nats.Conn) PubSub {
	return &natsPubSub{natsConn}
}

func (n *natsPubSub) Publish(topic string, message []byte) error {
	logger.Debug("[NATS] Publishing message", "topic", topic)
	return n.natsConn.Publish(topic, message)
}

func (n *natsPubSub) PublishWithReply(topic, reply string, data []byte) error {
	return n.natsConn.PublishMsg(&nats.Msg{
		Subject: topic,
		Reply:   reply,
		Data:    data,
	})
}

func (n *natsPubSub) Subscribe(topic string, handler func(msg *nats.Msg)) (Subscription, error) {
	// TODO: Handle subscription
	// handle more fields in msg
	sub, err := n.natsConn.Subscribe(topic, func(msg *nats.Msg) {
		handler(msg)
	})
	if err != nil {
		return nil, err
	}

	return &natsSubscription{subscription: sub}, nil
}

type DurablePubsub interface {
	Publish(topic string, message []byte) error
	Subscribe(handler func(msg jetstream.Msg)) (Subscription, error)
	Close() error
}

type jetStreamPubSub struct {
	js     jetstream.JetStream
	config jetstream.StreamConfig
}

type durablePubsub struct {
	consumerName    string
	js              jetstream.JetStream
	consumer        jetstream.Consumer
	consumerContext jetstream.ConsumeContext
}

func NewJetStreamPubsubManager(natsConn *nats.Conn, streamName string, subjects []string) *jetStreamPubSub {
	js, err := jetstream.New(natsConn)
	if err != nil {
		logger.Fatal("Error creating JetStream context: ", err)
	}

	ctx := context.Background()
	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		logger.Warn("Stream not found, creating new stream", "stream", streamName)
	}
	if stream != nil {
		info, _ := stream.Info(ctx)
		logger.Debug("Stream found", "info", info)
	}

	config := jetstream.StreamConfig{
		Name:        streamName,
		Description: "Stream for " + streamName,
		Subjects:    subjects,
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.InterestPolicy, // Manage retetion based on consumer handling
	}
	_, err = js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:        streamName,
		Description: "Stream Pubsub for " + streamName,
		Subjects:    subjects,
		MaxAge:      3 * time.Minute,
	})

	if err != nil {
		logger.Fatal("Error creating JetStream stream: ", err)
	}
	logger.Info("Creating NATs Jetstream PubSub context successfully!")

	return &jetStreamPubSub{
		js:     js,
		config: config,
	}
}

func (j *jetStreamPubSub) RegisterDurablePubsubConsumer(consumerName, topic string) *durablePubsub {
	consumerConfig := jetstream.ConsumerConfig{
		Name:           sanitizeConsumerName(consumerName),
		Durable:        sanitizeConsumerName(consumerName),
		AckPolicy:      jetstream.AckExplicitPolicy,
		MaxDeliver:     4,
		FilterSubjects: []string{topic},
		BackOff:        []time.Duration{30 * time.Second, 30 * time.Second, 30 * time.Second},
		DeliverPolicy:  jetstream.DeliverAllPolicy, // Deliver all messages
		AckWait:        30 * time.Second,           // explicitly set ack wait here
	}

	logger.Info("Creating consumer", "config", consumerConfig, "stream", j.config.Name)
	consumer, err := j.js.CreateOrUpdateConsumer(context.Background(), j.config.Name, consumerConfig)

	if err != nil {
		logger.Error("❌ Failed to create or update consumer:", err)
		return nil
	}
	if consumer != nil {
		logger.Info("Successfully created or updated consumer", "consumer", consumer)
	}

	// Create a durable pubsub consumer
	durable := &durablePubsub{
		consumerName: consumerName,
		js:           j.js,
		consumer:     consumer,
	}

	return durable
}

func (d *durablePubsub) Publish(topic string, message []byte) error {
	_, err := d.js.Publish(context.Background(), topic, message)
	return err
}

func sanitizeConsumerName(name string) string {
	// Replace invalid characters
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ">", "all")
	name = strings.ReplaceAll(name, "*", "any")

	// Ensure it starts with a letter or underscore
	if len(name) > 0 && !unicode.IsLetter(rune(name[0])) && name[0] != '_' {
		name = "_" + name
	}

	return name
}

func (d *durablePubsub) Subscribe(handler func(msg jetstream.Msg)) (Subscription, error) {
	consumerContext, err := d.consumer.Consume(handler)
	if err != nil {
		logger.Error("❌ Failed to create consumer context:", err)
		return nil, err
	}

	logger.Info("Successfully created consumer context", "consumerName", d.consumerName)
	d.consumerContext = consumerContext

	// Create a subscription
	subscription := &jetstreamSubscription{
		consumer: d.consumer,
	}
	return subscription, nil

}

func (d *durablePubsub) Close() error {
	if d.consumerContext != nil {
		d.consumerContext.Stop()
		logger.Info("Successfully closed consumer context", "consumerName", d.consumerName)
	}

	return nil
}
