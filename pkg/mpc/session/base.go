package session

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"sync"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/keyinfo"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/party"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/nats-io/nats.go"
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
	StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte))
	StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte))

	GetSaveData() ([]byte, error)
	GetPublicKey(data []byte) []byte
	VerifySignature(msg []byte, signature []byte) (*common.SignatureData, error)

	Send(msg tss.Message)
	Listen(nodeID string)
	SaveKey(participantPeerIDs []string, threshold int, isReshared bool, data []byte) (err error)
	ErrCh() chan error
}

type session struct {
	walletID string
	party    party.PartyInterface

	broadcastSub messaging.Subscription
	directSub    messaging.Subscription
	pubSub       messaging.PubSub
	direct       messaging.DirectMessaging

	identityStore identity.Store
	kvstore       kvstore.KVStore
	keyinfoStore  keyinfo.Store

	topicComposer *TopicComposer
	composeKey    KeyComposerFn

	mu    sync.Mutex
	errCh chan error
}

func NewSession(
	curve Curve,
	purpose Purpose,
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	identityStore identity.Store,
	kvstore kvstore.KVStore,
	keyinfoStore keyinfo.Store,
) *session {
	errCh := make(chan error, 1000)
	return &session{
		walletID:      walletID,
		pubSub:        pubSub,
		direct:        direct,
		identityStore: identityStore,
		kvstore:       kvstore,
		keyinfoStore:  keyinfoStore,
		errCh:         errCh,
	}
}

func (s *session) ErrCh() chan error {
	return s.errCh
}

func (s *session) Send(msg tss.Message) {
	data, routing, err := msg.WireBytes()
	if err != nil {
		s.errCh <- fmt.Errorf("failed to wire bytes: %w", err)
		return
	}

	tssMsg := types.NewTssMessage(s.walletID, data, routing.IsBroadcast, routing.From, routing.To)
	signature, err := s.identityStore.SignMessage(&tssMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to sign message: %w", err)
		return
	}
	tssMsg.Signature = signature
	msgBytes, err := types.MarshalTssMessage(&tssMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to marshal message: %w", err)
		return
	}

	logger.Debug("Sending message", "from", routing.From, "to", routing.To, "isBroadcast", routing.IsBroadcast)

	if routing.IsBroadcast && len(routing.To) == 0 {
		err := s.pubSub.Publish(s.topicComposer.ComposeBroadcastTopic(), msgBytes)
		if err != nil {
			s.errCh <- fmt.Errorf("failed to publish message: %w", err)
			return
		}
	} else {
		for _, to := range routing.To {
			nodeID := partyIDToNodeID(to)
			topic := s.topicComposer.ComposeDirectTopic(nodeID)
			err := s.direct.Send(topic, msgBytes)
			if err != nil {
				s.errCh <- fmt.Errorf("failed to send message: %w", err)
				return
			}
		}
	}
}

func (s *session) receive(rawMsg []byte) {
	msg, err := types.UnmarshalTssMessage(rawMsg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to unmarshal message: %w", err)
		return
	}

	err = s.identityStore.VerifyMessage(msg)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to verify message: %w", err)
		return
	}

	// Skip messages from self
	if msg.From.String() == s.party.PartyID().String() {
		return
	}

	toIDs := make([]string, len(msg.To))
	for i, id := range msg.To {
		toIDs[i] = id.String()
	}

	isBroadcast := msg.IsBroadcast && len(msg.To) == 0
	isToSelf := slices.Contains(toIDs, s.party.PartyID().String())

	if isBroadcast || isToSelf {
		logger.Debug("Received message", "from", msg.From, "to", msg.To, "isBroadcast", msg.IsBroadcast, "isToSelf", isToSelf)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.party.InCh() <- *msg
	}
}

func (s *session) Listen(nodeID string) {
	broadcast := func() {
		sub, err := s.pubSub.Subscribe(s.topicComposer.ComposeBroadcastTopic(), func(natMsg *nats.Msg) {
			msg := natMsg.Data
			s.receive(msg)
		})

		if err != nil {
			s.errCh <- fmt.Errorf("failed to subscribe to broadcast topic %s: %w", s.topicComposer.ComposeBroadcastTopic(), err)
			return
		}

		s.broadcastSub = sub
	}

	direct := func() {
		sub, err := s.direct.Listen(s.topicComposer.ComposeDirectTopic(fmt.Sprintf("%s:%s", nodeID, "keygen")), func(msg []byte) {
			s.receive(msg)
		})

		if err != nil {
			s.errCh <- fmt.Errorf("failed to subscribe to direct topic %s: %w", s.topicComposer.ComposeDirectTopic(s.party.PartyID().String()), err)
			return
		}

		s.directSub = sub
	}

	go broadcast()
	go direct()
}

func (s *session) SaveKey(participantPeerIDs []string, threshold int, isReshared bool, data []byte) (err error) {
	keyInfo := keyinfo.KeyInfo{
		ParticipantPeerIDs: participantPeerIDs,
		Threshold:          threshold,
		IsReshared:         isReshared,
	}
	composeKey := s.composeKey(s.walletID)
	err = s.keyinfoStore.Save(composeKey, &keyInfo)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to save keyinfo: %w", err)
		return
	}

	err = s.kvstore.Put(composeKey, data)
	if err != nil {
		s.errCh <- fmt.Errorf("failed to save key: %w", err)
		return
	}
	logger.Info("Saved key", "walletID", s.walletID, "threshold", threshold, "isReshared", isReshared, "data", len(data))
	return nil
}

func (s *session) GetSaveData() ([]byte, error) {
	composeKey := s.composeKey(s.walletID)
	data, err := s.kvstore.Get(composeKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}
	return data, nil
}
