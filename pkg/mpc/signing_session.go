package mpc

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/common"
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/common/errors"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
)

const (
	TypeSignSuccess = "mpc:sign:success"
)

type SigningSession struct {
	Session
	endCh               chan common.SignatureData
	data                *keygen.LocalPartySaveData
	tx                  *big.Int
	txID                string
	networkInternalCode string
}

type SigningSuccessEvent struct {
	NetworkInternalCode string `json:"network_internal_code"`
	WalletID            string `json:"wallet_id"`
	TxID                string `json:"tx_id"`
	R                   []byte `json:"r"`
	S                   []byte `json:"s"`
	SignatureRecovery   []byte `json:"signature_recovery"`
}

func NewSigningSession(
	walletID string,
	txID string,
	networkInternalCode string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	preParams *keygen.LocalPreParams,
	kvstore kvstore.KVStore,
) *SigningSession {
	return &SigningSession{
		Session: Session{
			walletID:    walletID,
			pubSub:      pubSub,
			direct:      direct,
			threshold:   threshold,
			selfPartyID: selfID,
			partyIDs:    partyIDs,
			outCh:       make(chan tss.Message),
			ErrCh:       make(chan error),
			preParams:   preParams,
			kvstore:     kvstore,
		},
		endCh:               make(chan common.SignatureData),
		txID:                txID,
		networkInternalCode: networkInternalCode,
	}
}

func (s *SigningSession) Init(tx *big.Int) {
	logger.Infof("Initializing signing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.S256(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)

	val, err := s.kvstore.Get(s.walletID)
	if err != nil {
		s.ErrCh <- errors.Wrap(err, "Failed to get wallet data from KVStore")
		return
	}

	var data keygen.LocalPartySaveData
	err = json.Unmarshal(val, &data)
	if err != nil {
		s.ErrCh <- errors.Wrap(err, "Failed to unmarshal wallet data")
		return
	}

	s.party = signing.NewLocalParty(tx, params, data, s.outCh, s.endCh)
	s.data = &data
	s.tx = tx
	logger.Info("Initialized sigining session successfully!")
}

func (s *SigningSession) Sign() {
	logger.Info("Starting signing", "walletID", s.walletID)
	go func() {
		if err := s.party.Start(); err != nil {
			s.ErrCh <- err
		}
	}()

	for {

		select {
		case msg := <-s.outCh:
			logger.Info("Generating and handle tss message", "walletID", s.walletID)
			s.handleTssMessage(msg)
		case sig := <-s.endCh:
			publicKey := *s.data.ECDSAPub
			pk := ecdsa.PublicKey{
				Curve: publicKey.Curve(),
				X:     publicKey.X(),
				Y:     publicKey.Y(),
			}

			ok := ecdsa.Verify(&pk, s.tx.Bytes(), new(big.Int).SetBytes(sig.R), new(big.Int).SetBytes(sig.S))
			if !ok {
				s.ErrCh <- errors.New("Failed to verify signature")
				return
			}

			r := SigningSuccessEvent{
				NetworkInternalCode: s.networkInternalCode,
				WalletID:            s.walletID,
				TxID:                s.txID,
				R:                   sig.R,
				S:                   sig.S,
				SignatureRecovery:   sig.SignatureRecovery,
			}

			bytes, err := json.Marshal(r)
			if err != nil {
				s.ErrCh <- errors.Wrap(err, "Failed to marshal raw signature")
				return
			}

			err = s.pubSub.Publish(TypeSignSuccess, bytes)
			if err != nil {
				s.ErrCh <- errors.Wrap(err, "Failed to publish sign success message")
				return
			}

			logger.Info("Sign successfully", "walletID", s.walletID)
		}

	}
}

func (s *SigningSession) composeDirectMessageTopic(nodeID string) string {
	return fmt.Sprintf("sign:direct:%s:%s", s.walletID, nodeID)
}

func (s *SigningSession) composeBroadcastTopic() string {
	return fmt.Sprintf("sign:broadcast:%s", s.walletID)
}

func (s *SigningSession) handleTssMessage(tssMsg tss.Message) {
	data, routing, err := tssMsg.WireBytes()
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
		logger.Info("Broadcasting message", "walletID", s.walletID, "from", s.selfPartyID, "routing", routing.To)
		err := s.pubSub.Publish(s.composeBroadcastTopic(), msg)
		if err != nil {
			s.ErrCh <- err
			return
		}
	} else {
		for _, to := range routing.To {
			nodeID := PartyIDToNodeID(to)
			topic := s.composeDirectMessageTopic(nodeID)
			logger.Info("Direct message", "walletID", s.walletID, "from", s.selfPartyID, "to", to)
			logger.Info("Sending direct message to destination topic", "topic", topic)
			err := s.direct.Send(topic, msg)
			if err != nil {
				s.ErrCh <- fmt.Errorf("Failed to send direct message to %s: %w", topic, err)
			}

		}

	}
}

func (s *SigningSession) ListenToIncomingMessage() {
	go func() {
		s.pubSub.Subscribe(s.composeBroadcastTopic(), func(msg []byte) {
			logger.Info("Received tss message from broadcast channel")
			s.receiveTssMessage(msg)
		})

	}()

	nodeID := PartyIDToNodeID(s.selfPartyID)
	logger.Info("Listening on target topic for incoming messages", "topic", s.composeDirectMessageTopic(nodeID))
	targetID := s.composeDirectMessageTopic(nodeID)
	s.direct.Listen(targetID, func(msg []byte) {
		logger.Info("Received message from direct channel", "targetID", targetID)
		s.receiveTssMessage(msg)
	})

}

func (s *SigningSession) receiveTssMessage(rawMsg []byte) {
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
