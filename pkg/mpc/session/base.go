package session

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/infra"
	"github.com/fystack/mpcium/pkg/keyinfo"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/party"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/hashicorp/consul/api"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Curve string

type Purpose string

const (
	CurveSecp256k1 Curve = "secp256k1"
	CurveEd25519   Curve = "ed25519"

	PurposeKeygen  Purpose = "keygen"
	PurposeSign    Purpose = "sign"
	PurposeReshare Purpose = "reshare"
)

type TopicComposer struct {
	ComposeBroadcastTopic func() string
	ComposeDirectTopic    func(nodeID string) string
}

type KeyComposerFn func(id string) string

type Session interface {
	StartKeygen(ctx context.Context, send func(tss.Message), onComplete func([]byte))
	StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), onComplete func([]byte))
	StartResharing(
		ctx context.Context,
		oldPartyIDs []*tss.PartyID,
		newPartyIDs []*tss.PartyID,
		oldThreshold int,
		newThreshold int,
		send func(tss.Message),
		onComplete func([]byte),
	)

	GetSaveData(version int) ([]byte, error)
	GetPublicKey(data []byte) ([]byte, error)
	VerifySignature(msg []byte, signature []byte) (*common.SignatureData, error)

	PartyIDs() []*tss.PartyID
	Send(msg tss.Message)
	Listen(ctx context.Context)
	SaveKey(participantPeerIDs []string, threshold int, version int, data []byte) (err error)
	WaitForReady(ctx context.Context, sessionID string) error
	ErrCh() chan error
	Close()
}

type session struct {
	walletID string
	party    party.Party
	curve    Curve

	broadcastSub messaging.Subscription
	directSub    messaging.Subscription
	pubSub       messaging.PubSub
	direct       messaging.DirectMessaging

	identityStore identity.Store
	kvstore       kvstore.KVStore
	keyinfoStore  keyinfo.Store

	topicComposer *TopicComposer
	composeKey    KeyComposerFn
	consulKV      infra.ConsulKV

	mu          sync.Mutex
	errCh       chan error
	msgBuffer   chan *nats.Msg
	workerCount int

	ctx    context.Context
	tracer trace.Tracer
}

func NewSession(
	curve Curve,
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	identityStore identity.Store,
	kvstore kvstore.KVStore,
	keyinfoStore keyinfo.Store,
	consulKV infra.ConsulKV,
) *session {
	errCh := make(chan error, 1000)
	return &session{
		curve:         curve,
		walletID:      walletID,
		pubSub:        pubSub,
		direct:        direct,
		identityStore: identityStore,
		kvstore:       kvstore,
		keyinfoStore:  keyinfoStore,
		errCh:         errCh,
		consulKV:      consulKV,
		tracer:        otel.Tracer("mpcium/session"),
		msgBuffer:     make(chan *nats.Msg, 100), // Buffer for 100 messages
	}
}

func (s *session) PartyIDs() []*tss.PartyID {
	return s.party.PartyIDs()
}

func (s *session) ErrCh() chan error {
	return s.errCh
}

func (s *session) WaitForReady(ctx context.Context, sessionID string) error {
	// build our Consul prefix
	prefix := fmt.Sprintf("tss-ready/%s/%s/", s.walletID, sessionID)

	// 1) publish our ready flag
	myKey := prefix + s.party.PartyID().String()
	if _, err := s.consulKV.Put(&api.KVPair{
		Key:   myKey,
		Value: []byte("true"),
	}, nil); err != nil {
		return fmt.Errorf("failed to write ready flag: %w", err)
	}

	// 2) poll until we see everyone
	total := len(s.party.PartyIDs())
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pairs, _, err := s.consulKV.List(prefix, nil)
			if err != nil {
				logger.Error("error listing readiness keys", err)
				continue
			}
			if len(pairs) >= total {
				logger.Info("[READY] peers ready", "have", len(pairs), "need", total, "walletID", s.walletID)
				return nil
			}
			// logger.Info("[READY]  Waiting for peers ready", "wallet", s.walletID, "have", len(pairs), "need", total)
		}
	}
}

// Send is a wrapper around the party's Send method
// It signs the message and sends it to the remote party
func (s *session) Send(msg tss.Message) {
	data, routing, err := msg.WireBytes()
	if err != nil {
		s.errCh <- fmt.Errorf("failed to wire bytes: %w", err)
		return
	}

	// Create a more descriptive span name from the message type
	msgType, err := s.party.GetMsgType(data)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to get message type: %w", err)
		return
	}
	parts := strings.Split(msgType, ".")
	spanName := fmt.Sprintf("session.Send: %s: %s", s.curve, parts[len(parts)-1])

	_, span := s.tracer.Start(s.ctx, spanName)
	defer span.End()

	tssMsg := types.NewTssMessage(s.walletID, data, routing.IsBroadcast, routing.From, routing.To)
	signature, err := s.identityStore.SignMessage(&tssMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to sign message: %w", err)
		span.RecordError(err)
		return
	}
	tssMsg.Signature = signature
	msgBytes, err := types.MarshalTssMessage(&tssMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to marshal message: %w", err)
		span.RecordError(err)
		return
	}
	round, _, err := s.party.ClassifyMsg(data)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to classify message: %w", err)
		span.RecordError(err)
		return
	}
	toNodeIDs := make([]string, len(routing.To))
	for i, to := range routing.To {
		toNodeIDs[i] = getRoutingFromPartyID(to)
	}
	logger.Debug("Sending message", "from", routing.From.Moniker, "to", toNodeIDs, "isBroadcast", routing.IsBroadcast, "round", round)
	span.SetAttributes(
		attribute.String("from", routing.From.Moniker),
		attribute.StringSlice("to", toNodeIDs),
		attribute.Bool("is_broadcast", routing.IsBroadcast),
		attribute.String("round", strconv.Itoa(int(round))),
		attribute.String("timestamp", time.Now().Format(time.RFC3339Nano)),
	)

	logger.Info(
		"Sending",
		"round",
		parts[len(parts)-1],
		"isBroadcast",
		routing.IsBroadcast,
		"receivers",
		toNodeIDs,
	)

	if routing.IsBroadcast && len(routing.To) == 0 {
		err := s.pubSub.Publish(s.ctx, s.topicComposer.ComposeBroadcastTopic(), msgBytes)
		if err != nil {
			s.errCh <- fmt.Errorf("failed to publish message: %w", err)
			span.RecordError(err)
			return
		}
		span.SetAttributes(attribute.String("topic", s.topicComposer.ComposeBroadcastTopic()))
	} else {
		for _, to := range routing.To {
			nodeID := getRoutingFromPartyID(to)
			topic := s.topicComposer.ComposeDirectTopic(nodeID)
			err := s.direct.Send(s.ctx, topic, msgBytes)
			span.SetAttributes(attribute.String("topic", topic))
			if err != nil {
				s.errCh <- fmt.Errorf("failed to send message: %w", err)
				span.RecordError(err)
				return
			}
		}
	}
}

// Listen is a wrapper around the party's Listen method
// It subscribes to the broadcast and self direct topics
func (s *session) Listen(ctx context.Context) {
	spanName := fmt.Sprintf("session.Listen:%s", s.curve)
	listenCtx, span := s.tracer.Start(ctx, spanName)
	defer span.End()

	span.SetAttributes(
		attribute.String("timestamp", time.Now().Format(time.RFC3339Nano)),
	)

	// Start a single worker to process messages sequentially
	go s.worker(listenCtx)

	var wg sync.WaitGroup
	wg.Add(2)

	selfDirectTopic := s.topicComposer.ComposeDirectTopic(getRoutingFromPartyID(s.party.PartyID()))
	broadcastTopic := s.topicComposer.ComposeBroadcastTopic()

	broadcast := func() {
		defer wg.Done()

		_, bSpan := s.tracer.Start(listenCtx, "session.Listen.Subscribe.Broadcast")
		defer bSpan.End()
		bSpan.SetAttributes(
			attribute.String("topic", broadcastTopic),
			attribute.String("timestamp", time.Now().Format(time.RFC3339Nano)),
		)

		sub, err := s.pubSub.Subscribe(broadcastTopic, func(natMsg *nats.Msg) {
			s.msgBuffer <- natMsg
		})

		if err != nil {
			bSpan.RecordError(err)
			s.errCh <- fmt.Errorf("failed to subscribe to broadcast topic %s: %w", s.topicComposer.ComposeBroadcastTopic(), err)
			return
		}

		s.broadcastSub = sub
	}

	direct := func() {
		defer wg.Done()

		_, dSpan := s.tracer.Start(listenCtx, "session.Listen.Subscribe.Direct")
		defer dSpan.End()
		dSpan.SetAttributes(
			attribute.String("topic", selfDirectTopic),
			attribute.String("timestamp", time.Now().Format(time.RFC3339Nano)),
		)

		sub, err := s.direct.Listen(selfDirectTopic, func(natMsg *nats.Msg) {
			s.msgBuffer <- natMsg
		})

		if err != nil {
			dSpan.RecordError(err)
			s.errCh <- fmt.Errorf("failed to subscribe to direct topic %s: %w", s.topicComposer.ComposeDirectTopic(s.party.PartyID().String()), err)
			return
		}

		s.directSub = sub
	}

	go broadcast()
	go direct()
	wg.Wait()
}

// SaveKey saves the key to the keyinfo store and the kvstore
func (s *session) SaveKey(participantPeerIDs []string, threshold int, version int, data []byte) (err error) {
	keyInfo := keyinfo.KeyInfo{
		ParticipantPeerIDs: participantPeerIDs,
		Threshold:          threshold,
		Version:            version,
	}
	composeKey := s.composeKey(s.walletID)
	err = s.keyinfoStore.Save(composeKey, &keyInfo)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to save keyinfo: %w", err)
		return
	}

	err = s.kvstore.Put(fmt.Sprintf("%s-%d", composeKey, version), data)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to save key: %w", err)
		return
	}
	return
}

func (s *session) SetSaveData(saveBytes []byte) {
	s.party.SetSaveData(saveBytes)
}

// GetSaveData gets the key from the kvstore
func (s *session) GetSaveData(version int) ([]byte, error) {
	composeKey := s.composeKey(s.walletID)
	data, err := s.kvstore.Get(fmt.Sprintf("%s-%d", composeKey, version))
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	return data, nil
}

func (s *session) Close() {
	// Close subscriptions first
	if s.broadcastSub != nil {
		s.broadcastSub.Unsubscribe()
	}
	if s.directSub != nil {
		s.directSub.Unsubscribe()
	}

	// Close party
	if s.party != nil {
		s.party.Close()
	}

	// close msg buffer
	if s.msgBuffer != nil {
		close(s.msgBuffer)
	}

	// Close error channel last
	select {
	case <-s.errCh:
		// Channel already closed
	default:
		close(s.errCh)
	}
}

func (s *session) worker(ctx context.Context) {
	for natMsg := range s.msgBuffer {
		s.receive(ctx, natMsg)
	}
}

// receive is a helper function that receives a message from the party
func (s *session) receive(ctx context.Context, natMsg *nats.Msg) {
	carrier := messaging.NewNatsHeaderCarrier(natMsg.Header)
	parentCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)

	rawMsg := natMsg.Data
	msg, err := types.UnmarshalTssMessage(rawMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to unmarshal message: %w", err)
		return
	}

	// Create a more descriptive span name from the message type
	msgType, err := s.party.GetMsgType(msg.MsgBytes)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to get message type: %w", err)
		return
	}
	parts := strings.Split(msgType, ".")
	spanName := fmt.Sprintf("session.receive: %s: %s", s.curve, parts[len(parts)-1])

	_, span := s.tracer.Start(parentCtx, spanName)
	defer span.End()

	err = s.identityStore.VerifyMessage(msg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to verify message: %w", err)
		span.RecordError(err)
		return
	}

	// logger.Info(
	// 	"Message",
	// 	"from",
	// 	msg.From.String(),
	// 	"partyID",
	// 	s.party.PartyID().String(),
	// 	"Equal",
	// 	msg.From.String() == s.party.PartyID().String(),
	// )
	if msg.From.String() == s.party.PartyID().String() {
		return
	}

	toIDs := make([]string, len(msg.To))
	for i, id := range msg.To {
		toIDs[i] = id.String()
	}

	isBroadcast := msg.IsBroadcast
	isToSelf := slices.Contains(toIDs, s.party.PartyID().String())

	fromID := msg.From.Moniker // e.g. "7b1090cd-ffe3-46ff-8375-594dd3204169:keygen"
	fromIDParts := strings.SplitN(fromID, ":", 2)
	nodeID := fromIDParts[0]
	logger.Info(
		"Message",
		"from",
		nodeID,
		"curve",
		s.curve,
		"round",
		parts[len(parts)-1],
		"isBroadcast",
		isBroadcast,
		"isToSelf",
		isToSelf,
	) // Skip messages from self

	if isBroadcast || isToSelf {
		round, _, err := s.party.ClassifyMsg(msg.MsgBytes)
		if err != nil {
			s.errCh <- fmt.Errorf("failed to classify message: %w", err)
			span.RecordError(err)
			return
		}

		span.SetAttributes(
			attribute.String("from", msg.From.Moniker),
			attribute.String("round", strconv.Itoa(int(round))),
			attribute.Bool("is_broadcast", isBroadcast),
			attribute.Bool("is_to_self", isToSelf),
			attribute.String("timestamp", time.Now().Format(time.RFC3339Nano)),
		)

		logger.Debug("Received message", "from", msg.From.Moniker, "round", round, "isBroadcast", msg.IsBroadcast, "isToSelf", isToSelf)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.party.InCh() <- *msg
	}
}
