package mpc

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/encoding"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
)

const (
	TypeGenerateWalletSuccess = "mpc:generate:success:%s"
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
	endCh     chan keygen.LocalPartySaveData
	ErrCh     chan error
	party     tss.Party
	preParams *keygen.LocalPreParams
	kvstore   kvstore.KVStore
}

func NewSession(
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	preParams *keygen.LocalPreParams,
	kvstore kvstore.KVStore,
) *Session {
	return &Session{
		walletID:    walletID,
		pubSub:      pubSub,
		direct:      direct,
		threshold:   threshold,
		selfPartyID: selfID,
		partyIDs:    partyIDs,
		outCh:       make(chan tss.Message),
		endCh:       make(chan keygen.LocalPartySaveData),
		ErrCh:       make(chan error),
		preParams:   preParams,
		kvstore:     kvstore,
	}
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

func (s *Session) composeReadyTopic() string {
	return fmt.Sprintf("%s-%s", s.walletID, "ready")
}

func (s *Session) Init() {
	logger.Infof("Initializing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.S256(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)
	s.party = keygen.NewLocalParty(params, s.outCh, s.endCh, *s.preParams)
	logger.Info("Initialized session successfully")
	logger.Infof("PartyID: %s is ready!", s.party.PartyID())
}

func (s *Session) composeTargetID(nodeID string) string {
	return fmt.Sprintf("%s/%s", s.walletID, nodeID)
}

func (s *Session) handleTssMessage(keyshare tss.Message) {
	data, routing, err := keyshare.WireBytes()
	logger.Info("Received message", "routing", routing)
	if err != nil {
		s.ErrCh <- err
		return
	}

	msg, err := MarshalTssMessage(s.walletID, data, routing.IsBroadcast, routing.From, routing.To)
	if err != nil {
		s.ErrCh <- fmt.Errorf("failed to marshal tss message: %w", err)
		return
	}
	logger.Info("Preparing to send message to", "walletID", s.walletID, "to", routing.To, "isBroadcast", routing.IsBroadcast)
	if routing.IsBroadcast && len(routing.To) == 0 {
		logger.Info("Broadcasting message", "walletID", s.walletID, "from", s.PartyID(), "routing", routing.To)
		err := s.pubSub.Publish(s.walletID, msg)
		if err != nil {
			s.ErrCh <- err
			return
		}
	} else {
		for _, to := range routing.To {
			nodeID := PartyIDToNodeID(to)
			topic := s.composeTargetID(nodeID)
			logger.Info("Direct message", "walletID", s.walletID, "from", s.PartyID(), "to", to)
			logger.Info("Sending direct message to destination topic", "topic", topic)
			err := s.direct.Send(topic, msg)
			if err != nil {
				s.ErrCh <- fmt.Errorf("Failed to send direct message to %s: %w", topic, err)
			}

		}

	}
}

func (s *Session) GenerateKey() {
	logger.Info("Starting to generate key", "walletID", s.walletID)
	go func() {
		if err := s.party.Start(); err != nil {
			s.ErrCh <- err
		}
	}()

	for {
		select {
		case msg := <-s.outCh:
			logger.Info("Received message for session", "walletID", s.walletID, "msg", msg)
			s.handleTssMessage(msg)
		case saveData := <-s.endCh:
			keyBytes, err := json.Marshal(saveData)
			if err != nil {
				s.ErrCh <- err
				return
			}

			err = s.kvstore.Put(s.walletID, keyBytes)
			if err != nil {
				logger.Error("Failed to save key", err, "walletID", s.walletID)
				s.ErrCh <- err
				return
			}

			publicKey := saveData.ECDSAPub
			pubKey := &ecdsa.PublicKey{
				Curve: publicKey.Curve(),
				X:     publicKey.X(),
				Y:     publicKey.Y(),
			}

			pubKeyBytes, err := encoding.EncodeS256PubKey(pubKey)
			if err != nil {
				logger.Error("failed to encode public key", err)
				s.ErrCh <- fmt.Errorf("failed to encode public key: %w", err)
				return
			}
			// val, err := s.kvstore.Get(s.walletID)
			// if err != nil {
			// 	logger.Error("failed to get key from kvstore", err)
			// }

			// var data keygen.LocalPartySaveData

			// err = json.Unmarshal(val, &data)
			// if err != nil {
			// 	panic(err)
			// }

			s.pubSub.Publish(fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID), pubKeyBytes)
		}

	}

}

func (s *Session) ListenToIncomingMessage() {
	go func() {
		s.pubSub.Subscribe(s.walletID, func(msg []byte) {
			logger.Info("Received tss message from broadcast channel")
			s.receiveTssMessage(msg)
		})

	}()

	nodeID := PartyIDToNodeID(s.selfPartyID)
	logger.Info("Listening on target topic for incoming messages", "topic", s.composeTargetID(nodeID))
	targetID := s.composeTargetID(nodeID)
	s.direct.Listen(targetID, func(msg []byte) {
		logger.Info("Received message from direct channel", "targetID", targetID)

		s.receiveTssMessage(msg)
	})

}

func (s *Session) receiveTssMessage(rawMsg []byte) {
	msg, err := UnmarshalTssMessage(rawMsg)
	if err != nil {
		s.ErrCh <- fmt.Errorf("Failed to unmarshal message: %w", err)
		return
	}

	logger.Info("Received message", "from", msg.From, "to", msg.To, "isBroadcast", msg.IsBroadcast)
	isBroadcast := msg.IsBroadcast && len(msg.To) == 0
	isToSelf := len(msg.To) == 1 && ComparePartyIDs(msg.To[0], s.selfPartyID)

	if isBroadcast || isToSelf {
		go func() {
			if isBroadcast {
				logger.Info("Updating broadcast message", "to", msg.To)
			} else if isToSelf {
				logger.Info("Updating direct message to local node", "to", msg.To)
			}

			ok, err := s.party.UpdateFromBytes(msg.MsgBytes, msg.From, msg.IsBroadcast)
			if !ok || err != nil {
				logger.Error("Failed to update party", err, "walletID", s.walletID)
				return
			}

		}()
	}
}

// Close and clean up
func (s *Session) Close() {
	return
}
