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

type KeygenSession struct {
	Session
	endCh chan keygen.LocalPartySaveData
}

func NewKeygenSession(
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	preParams *keygen.LocalPreParams,
	kvstore kvstore.KVStore,
) *KeygenSession {
	return &KeygenSession{
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
		endCh: make(chan keygen.LocalPartySaveData),
	}
}

func (s *KeygenSession) PartyID() *tss.PartyID {
	return s.selfPartyID
}

func (s *KeygenSession) PartyIDs() []*tss.PartyID {
	return s.partyIDs
}

func (s *KeygenSession) PartyCount() int {
	return len(s.partyIDs)
}

func (s *KeygenSession) Init() {
	logger.Infof("Initializing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.S256(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)
	s.party = keygen.NewLocalParty(params, s.outCh, s.endCh, *s.preParams)
	logger.Infof("[INITIALIZED] Initialized session successfully partyID: %s, peerIDs %s, walletID %s", s.selfPartyID, s.partyIDs, s.walletID)
}

func (s *KeygenSession) composeDirectMessageTopic(nodeID string) string {
	return fmt.Sprintf("keygen:direct:%s:%s", s.walletID, nodeID)
}

func (s *KeygenSession) composeBroadcastTopic() string {
	return fmt.Sprintf("keygen:broadcast:%s", s.walletID)
}

func (s *KeygenSession) handleTssMessage(keyshare tss.Message) {
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
		err := s.pubSub.Publish(s.composeBroadcastTopic(), msg)
		if err != nil {
			s.ErrCh <- err
			return
		}
	} else {
		for _, to := range routing.To {
			nodeID := PartyIDToNodeID(to)
			topic := s.composeDirectMessageTopic(nodeID)
			err := s.direct.Send(topic, msg)
			if err != nil {
				s.ErrCh <- fmt.Errorf("Failed to send direct message to %s: %w", topic, err)
			}

		}

	}
}

func (s *KeygenSession) GenerateKey() {
	logger.Info("Starting to generate key", "walletID", s.walletID)
	go func() {
		if err := s.party.Start(); err != nil {
			s.ErrCh <- err
		}
	}()

	for {
		select {
		case msg := <-s.outCh:
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

			err = s.pubSub.Publish(fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID), pubKeyBytes)
			if err != nil {
				logger.Error("Failed to publish key generation success message", err)
				s.ErrCh <- fmt.Errorf("Failed to publish key generation success message %w", err)
				return
			}

			logger.Info("[COMPLETED] Key generation completed successfully", "walletID", s.walletID)
			err = s.Close()
			if err != nil {
				logger.Error("Failed to close session", err)
			}
			return
		}
	}
}

func (s *KeygenSession) ListenToIncomingMessage() {
	go func() {
		sub, err := s.pubSub.Subscribe(s.composeBroadcastTopic(), func(msg []byte) {
			s.receiveTssMessage(msg)
		})

		if err != nil {
			s.ErrCh <- fmt.Errorf("Failed to subscribe to broadcast topic %s: %w", s.composeBroadcastTopic(), err)
			return
		}

		s.broadcastSub = sub
	}()

	nodeID := PartyIDToNodeID(s.selfPartyID)
	targetID := s.composeDirectMessageTopic(nodeID)
	sub, err := s.direct.Listen(targetID, func(msg []byte) {
		go s.receiveTssMessage(msg) // async for avoid timeout
	})
	if err != nil {
		s.ErrCh <- fmt.Errorf("Failed to subscribe to direct topic %s: %w", targetID, err)
	}
	s.directSub = sub

}

// Close and clean up
func (s *KeygenSession) Close() error {
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
