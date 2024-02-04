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

	broadcastSub messaging.Subscription
	directSub    messaging.Subscription
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
