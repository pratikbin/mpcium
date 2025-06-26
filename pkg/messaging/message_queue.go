package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fystack/mpcium/pkg/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var (
	ErrPermament = errors.New("Permanent messaging error")
)

type MessageQueue interface {
	Enqueue(topic string, message []byte, options *EnqueueOptions) error
	Dequeue(topic string, handler func(message []byte) error) error
	Fetch(batch int, handler func(msg jetstream.Msg) error) error
	Close()
}

type EnqueueOptions struct {
	IdempotententKey string
}

type msgQueue struct {
	consumerName    string
	js              jetstream.JetStream
	consumer        jetstream.Consumer
	consumerContext jetstream.ConsumeContext
}

type msgPull struct {
	consumerName string
	js           jetstream.JetStream
	consumer     jetstream.Consumer
	fetchMaxWait time.Duration
}

type NATsMessageQueueManager struct {
	queueName string
	js        jetstream.JetStream
}

func NewNATsMessageQueueManager(queueName string, subjectWildCards []string, nc *nats.Conn) *NATsMessageQueueManager {
	js, err := jetstream.New(nc)
	if err != nil {
		logger.Fatal("Error creating JetStream context: ", err)
	}

	ctx := context.Background()
	stream, err := js.Stream(ctx, queueName)
	if err != nil {
		logger.Warn("Stream not found, creating new stream", "stream", queueName)
	}
	if stream != nil {
		info, _ := stream.Info(ctx)
		logger.Debug("Stream found", "info", info)

	}

	_, err = js.CreateOrUpdateStream(context.Background(), jetstream.StreamConfig{
		Name:        queueName,
		Description: "Stream for " + queueName,
		Subjects:    subjectWildCards,
		MaxBytes:    100_000_000, // Light Production (Low Traffic) 100_000_000 (100 MB)
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy,
	})
	if err != nil {
		logger.Fatal("Error creating JetStream stream: ", err)
	}
	logger.Info("Creating apex NATs Jetstream context successfully!")

	return &NATsMessageQueueManager{
		queueName: queueName,
		js:        js,
	}
}

func (m *NATsMessageQueueManager) NewMessageQueue(consumerName string) MessageQueue {
	mq := &msgQueue{
		consumerName: consumerName,
		js:           m.js,
	}
	consumerWildCard := fmt.Sprintf("%s.%s.*", m.queueName, consumerName)
	cfg := jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		MaxAckPending: 1000,
		// If a message isn't acked within AckWait, it will be redelivered up to MaxDelive
		AckWait:   60 * time.Second,
		AckPolicy: jetstream.AckExplicitPolicy,
		FilterSubjects: []string{
			consumerWildCard,
		},
		MaxDeliver: 3,
	}
	logger.Info("Creating consumer for subject", "config", cfg)
	consumer, err := m.js.CreateOrUpdateConsumer(context.Background(), m.queueName, cfg)
	if err != nil {
		logger.Fatal("Error creating JetStream consumer: ", err)
	}

	mq.consumer = consumer
	return mq
}

func (mq *msgQueue) Enqueue(topic string, message []byte, options *EnqueueOptions) error {
	header := nats.Header{}
	if options != nil {
		header.Add("Nats-Msg-Id", options.IdempotententKey)
	}

	logger.Info("Publishing message", "topic", topic)
	_, err := mq.js.PublishMsg(context.Background(), &nats.Msg{
		Subject: topic,
		Data:    message,
		Header:  header,
	})

	if err != nil {
		return fmt.Errorf("Error enqueueing message: %w", err)
	}

	return nil
}

func (mq *msgQueue) Dequeue(topic string, handler func(message []byte) error) error {
	c, err := mq.consumer.Consume(func(msg jetstream.Msg) {
		meta, _ := msg.Metadata()
		logger.Debug("Received message", "meta", meta)
		err := handler(msg.Data())
		if err != nil {
			if errors.Is(err, ErrPermament) {
				logger.Info("Permanent error on message", "meta", meta)
				msg.Term()
				return
			}

			logger.Error("Error handling message: ", err)
			msg.Nak()
			return
		}

		logger.Debug("Message Acknowledged", "meta", meta)
		err = msg.Ack()
		if err != nil {
			logger.Error("Error acknowledging message: ", err)
		}
	})
	mq.consumerContext = c
	return err
}

func (mq *msgQueue) Close() {
	// only close consumer if it was created - dequeue
	if mq.consumerContext != nil {
		mq.consumerContext.Stop()
	}
}

func (m *NATsMessageQueueManager) NewMessagePullSubscriber(consumerName string) MessageQueue {
	mq := &msgQueue{
		consumerName: consumerName,
		js:           m.js,
	}
	consumerWildCard := fmt.Sprintf("%s.%s.*", m.queueName, consumerName)
	cfg := jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		MaxAckPending: 1000,
		// If a message isn't acked within AckWait, it will be redelivered up to MaxDelive
		AckWait:    180 * time.Second,
		MaxWaiting: 1000,
		AckPolicy:  jetstream.AckExplicitPolicy,
		FilterSubjects: []string{
			consumerWildCard,
		},
		MaxDeliver:      1,
		MaxRequestBatch: 10,
	}

	logger.Info("Creating pull consumer for subject", "config", cfg)
	consumer, err := m.js.CreateOrUpdateConsumer(context.Background(), m.queueName, cfg)
	if err != nil {
		logger.Fatal("Error creating JetStream consumer: ", err)
	}

	mq.consumer = consumer
	return mq
}

func (mq *msgQueue) Fetch(batch int, handler func(msg jetstream.Msg) error) error {
	// Reduced fetch timeout from 2 minutes to 30 seconds for faster processing
	msgs, err := mq.consumer.Fetch(batch, jetstream.FetchMaxWait(30*time.Second))
	if err != nil {
		return fmt.Errorf("error fetching messages: %w", err)
	}

	for msg := range msgs.Messages() {
		meta, _ := msg.Metadata()
		logger.Debug("Received message", "meta", meta) // Changed to Debug to reduce log noise
		err := handler(msg)
		if err != nil {
			if errors.Is(err, ErrPermament) {
				logger.Info("Permanent error on message", "subject", msg.Subject)
				msg.Term()
				continue
			}

			logger.Error("Error handling message: ", err)
			msg.Nak()
			continue
		}

		err = msg.Ack()
		if err != nil {
			if !IsAlreadyAcknowledged(err) {
				logger.Error("Error acknowledging message:", err)
			}
		}
	}
	return nil
}

func IsAlreadyAcknowledged(err error) bool {
	return err != nil && strings.Contains(err.Error(), nats.ErrMsgAlreadyAckd.Error())
}
