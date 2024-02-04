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
	SignSuccessTopic = "mpc.mpc_sign_success.completed"
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
	succesQueue messaging.MessageQueue,
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
			topicComposer: &TopicComposer{
				ComposeBroadcastTopic: func() string {
					return fmt.Sprintf("sign:broadcast:%s", walletID)
				},
				ComposeDirectTopic: func(nodeID string) string {
					return fmt.Sprintf("sign:direct:%s:%s", walletID, nodeID)
				},
			},
			successQueue: succesQueue,
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

func (s *SigningSession) Sign(done func()) {
	logger.Info("Starting signing", "walletID", s.walletID)
	go func() {
		if err := s.party.Start(); err != nil {
			s.ErrCh <- err
		}
	}()

	for {

		select {
		case msg := <-s.outCh:
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

			err = s.successQueue.Enqueue(SignSuccessTopic, bytes, &messaging.EnqueueOptions{
				IdempotententKey: s.txID,
			})
			if err != nil {
				s.ErrCh <- errors.Wrap(err, "Failed to publish sign success message")
				return
			}

			logger.Info("[SIGN] Sign successfully", "walletID", s.walletID)
			err = s.Close()
			if err != nil {
				logger.Error("Failed to close session", err)
			}

			done()
			return
		}

	}
}
