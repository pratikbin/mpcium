package mpc

import (
	"fmt"
	"strings"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/common/errors"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
)

type TopicComposer struct {
	ComposeBroadcastTopic func() string
	ComposeDirectTopic    func(nodeID string) string
}

type Session struct {
	walletID    string
	pubSub      messaging.PubSub
	direct      messaging.DirectMessaging
	threshold   int
	selfPartyID *tss.PartyID
	// IDs of all parties in the session including self
	partyIDs  []*tss.PartyID
	outCh     chan tss.Message
	ErrCh     chan error
	party     tss.Party
	preParams *keygen.LocalPreParams
	kvstore   kvstore.KVStore

	broadcastSub  messaging.Subscription
	directSub     messaging.Subscription
	topicComposer *TopicComposer
}

func (s *Session) PartyID() *tss.PartyID {
	return s.selfPartyID
}

func (s *Session) PartyIDs() []*tss.PartyID {
	return s.partyIDs
}

func (s *Session) PartyCount() int {
	return len(s.partyIDs)
}

func (s *Session) handleTssMessage(keyshare tss.Message) {
	data, routing, err := keyshare.WireBytes()
	if err != nil {
		s.ErrCh <- err
		return
	}

	msg, err := MarshalTssMessage(s.walletID, data, routing.IsBroadcast, routing.From, routing.To)
	if err != nil {
		s.ErrCh <- fmt.Errorf("failed to marshal tss message: %w", err)
		return
	}
	if routing.IsBroadcast && len(routing.To) == 0 {
		err := s.pubSub.Publish(s.topicComposer.ComposeBroadcastTopic(), msg)
		if err != nil {
			s.ErrCh <- err
			return
		}
	} else {
		for _, to := range routing.To {
			nodeID := PartyIDToNodeID(to)
			topic := s.topicComposer.ComposeDirectTopic(nodeID)
			err := s.direct.Send(topic, msg)
			if err != nil {
				s.ErrCh <- fmt.Errorf("Failed to send direct message to %s: %w", topic, err)
			}

		}

	}
}

func (s *Session) receiveTssMessage(rawMsg []byte) {
	msg, err := UnmarshalTssMessage(rawMsg)
	if err != nil {
		s.ErrCh <- fmt.Errorf("Failed to unmarshal message: %w", err)
		return
	}

	toIDs := make([]string, len(msg.To))
	for i, id := range msg.To {
		toIDs[i] = id.String()
	}

	round, err := GetMsgRound(msg.MsgBytes, s.selfPartyID, msg.IsBroadcast)
	if err != nil {
		s.ErrCh <- errors.Wrap(err, "Broken TSS Share")
		return
	}

	logger.Info("Received message", "from", msg.From.String(), "to", strings.Join(toIDs, ","), "isBroadcast", msg.IsBroadcast, "round", round.RoundMsg)
	isBroadcast := msg.IsBroadcast && len(msg.To) == 0
	isToSelf := len(msg.To) == 1 && ComparePartyIDs(msg.To[0], s.selfPartyID)

	if isBroadcast || isToSelf {
		ok, err := s.party.UpdateFromBytes(msg.MsgBytes, msg.From, msg.IsBroadcast)
		if !ok || err != nil {
			logger.Error("Failed to update party", err, "walletID", s.walletID)
			return
		}

	}
}

func (s *Session) ListenToIncomingMessage() {
	go func() {
		sub, err := s.pubSub.Subscribe(s.topicComposer.ComposeBroadcastTopic(), func(msg []byte) {
			s.receiveTssMessage(msg)
		})

		if err != nil {
			s.ErrCh <- fmt.Errorf("Failed to subscribe to broadcast topic %s: %w", s.topicComposer.ComposeBroadcastTopic(), err)
			return
		}

		s.broadcastSub = sub
	}()

	nodeID := PartyIDToNodeID(s.selfPartyID)
	targetID := s.topicComposer.ComposeDirectTopic(nodeID)
	sub, err := s.direct.Listen(targetID, func(msg []byte) {
		go s.receiveTssMessage(msg) // async for avoid timeout
	})
	if err != nil {
		s.ErrCh <- fmt.Errorf("Failed to subscribe to direct topic %s: %w", targetID, err)
	}
	s.directSub = sub

}

func (s *Session) Close() error {
	err := s.broadcastSub.Unsubscribe()
	if err != nil {
		return err
	}
	err = s.directSub.Unsubscribe()
	if err != nil {
		return err
	}

	return nil
}
