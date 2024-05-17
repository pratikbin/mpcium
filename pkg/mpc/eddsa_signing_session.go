package mpc

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/common"
	"github.com/bnb-chain/tss-lib/v2/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/eddsa/signing"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/cryptoniumX/mpcium/pkg/common/errors"
	"github.com/cryptoniumX/mpcium/pkg/keyinfo"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/samber/lo"
)

type EDDSASigningSession struct {
	Session
	endCh               chan *common.SignatureData
	data                *keygen.LocalPartySaveData
	tx                  *big.Int
	txID                string
	networkInternalCode string
}

func NewEDDSASigningSession(
	walletID string,
	txID string,
	networkInternalCode string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	participantPeerIDs []string,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	kvstore kvstore.KVStore,
	keyinfoStore keyinfo.Store,
	succesQueue messaging.MessageQueue,
) *EDDSASigningSession {
	return &EDDSASigningSession{
		Session: Session{
			walletID:           walletID,
			pubSub:             pubSub,
			direct:             direct,
			threshold:          threshold,
			participantPeerIDs: participantPeerIDs,
			selfPartyID:        selfID,
			partyIDs:           partyIDs,
			outCh:              make(chan tss.Message),
			ErrCh:              make(chan error),
			// preParams:          preParams,
			kvstore:      kvstore,
			keyinfoStore: keyinfoStore,
			topicComposer: &TopicComposer{
				ComposeBroadcastTopic: func() string {
					return fmt.Sprintf("sign:eddsa:broadcast:%s", walletID)
				},
				ComposeDirectTopic: func(nodeID string) string {
					return fmt.Sprintf("sign:eddsa:direct:%s:%s", walletID, nodeID)
				},
			},
			composeKey: func(waleltID string) string {
				return fmt.Sprintf("eddsa:%s", waleltID)
			},
			successQueue: succesQueue,
		},
		endCh:               make(chan *common.SignatureData),
		txID:                txID,
		networkInternalCode: networkInternalCode,
	}
}

func (s *EDDSASigningSession) Init(tx *big.Int) error {
	logger.Infof("Initializing signing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)

	keyData, err := s.kvstore.Get(s.composeKey(s.walletID))
	if err != nil {
		return errors.Wrap(err, "Failed to get wallet data from KVStore")
	}

	keyInfo, err := s.keyinfoStore.Get(s.composeKey(s.walletID))
	if err != nil {
		return errors.Wrap(err, "Failed to get key info data")
	}

	if len(s.participantPeerIDs) < keyInfo.Threshold+1 {
		return fmt.Errorf("Not enough participants to sign, expected %d, got %d", keyInfo.Threshold+1, len(s.participantPeerIDs))
	}

	// check if t+1 participants are present
	result := lo.Intersect(s.participantPeerIDs, keyInfo.ParticipantPeerIDs)
	if len(result) < keyInfo.Threshold+1 {
		return fmt.Errorf(
			"Incompatible peerIDs to participate in signing. Current participants: %v, expected participants: %v",
			s.participantPeerIDs,
			keyInfo.ParticipantPeerIDs,
		)
	}

	logger.Info("Have enough participants to sign", "participants", s.participantPeerIDs)
	// Check if all the participants of the key are present
	var data keygen.LocalPartySaveData
	err = json.Unmarshal(keyData, &data)
	if err != nil {
		return errors.Wrap(err, "Failed to unmarshal wallet data")
	}

	s.party = signing.NewLocalParty(tx, params, data, s.outCh, s.endCh)
	s.data = &data
	s.tx = tx
	logger.Info("Initialized sigining session successfully!")
	return nil
}

func (s *EDDSASigningSession) Sign(done func()) {
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
			publicKey := *s.data.EDDSAPub
			pk := edwards.PublicKey{
				Curve: tss.Edwards(),
				X:     publicKey.X(),
				Y:     publicKey.Y(),
			}

			ok := edwards.Verify(&pk, s.tx.Bytes(), new(big.Int).SetBytes(sig.R), new(big.Int).SetBytes(sig.S))
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
