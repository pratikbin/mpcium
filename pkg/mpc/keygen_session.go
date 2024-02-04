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
	TypeGenerateWalletSuccess = "mpc.mpc_keygen_success.%s"
)

type KeygenSession struct {
	Session
	endCh chan keygen.LocalPartySaveData
}

type KeygenSuccessEvent struct {
	WalletID string `json:"wallet_id"`
	PubKey   []byte `json:"pub_key"`
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
	successQueue messaging.MessageQueue,
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
			topicComposer: &TopicComposer{
				ComposeBroadcastTopic: func() string {
					return fmt.Sprintf("keygen:broadcast:%s", walletID)
				},
				ComposeDirectTopic: func(nodeID string) string {
					return fmt.Sprintf("keygen:direct:%s:%s", walletID, nodeID)
				},
			},
			successQueue: successQueue,
		},
		endCh: make(chan keygen.LocalPartySaveData),
	}
}

func (s *KeygenSession) Init() {
	logger.Infof("Initializing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.S256(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)
	s.party = keygen.NewLocalParty(params, s.outCh, s.endCh, *s.preParams)
	logger.Infof("[INITIALIZED] Initialized session successfully partyID: %s, peerIDs %s, walletID %s", s.selfPartyID, s.partyIDs, s.walletID)
}

func (s *KeygenSession) GenerateKey(done func()) {
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

			successEvent := KeygenSuccessEvent{
				WalletID: s.walletID,
				PubKey:   pubKeyBytes,
			}

			successEventBytes, err := json.Marshal(successEvent)
			if err != nil {
				s.ErrCh <- fmt.Errorf("failed to marshal success event: %w", err)
				return
			}

			err = s.successQueue.Enqueue(fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID), successEventBytes, &messaging.EnqueueOptions{
				IdempotententKey: fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID),
			})
			if err != nil {
				logger.Error("Failed to publish key generation success message", err)
				s.ErrCh <- fmt.Errorf("Failed to publish key generation success message %w", err)
				return
			}

			logger.Info("[COMPLETED KEY GEN] Key generation completed successfully", "walletID", s.walletID)
			err = s.Close()
			if err != nil {
				logger.Error("Failed to close session", err)
			}
			done()
			return
		}
	}
}
