package mpc

import (
	"encoding/json"
	"fmt"

	"github.com/bnb-chain/tss-lib/v2/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/cryptoniumX/mpcium/pkg/keyinfo"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/decred/dcrd/dcrec/edwards/v2"
)

type EDDSAKeygenSession struct {
	Session
	endCh chan *keygen.LocalPartySaveData
}

type EDDSAKeygenSuccessEvent struct {
	WalletID string `json:"wallet_id"`
	PubKey   []byte `json:"pub_key"`
}

func NewEDDSAKeygenSession(
	walletID string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	participantPeerIDs []string,
	selfID *tss.PartyID,
	partyIDs []*tss.PartyID,
	threshold int,
	kvstore kvstore.KVStore,
	keyinfoStore keyinfo.Store,
	successQueue messaging.MessageQueue,
) *EDDSAKeygenSession {
	return &EDDSAKeygenSession{Session: Session{
		walletID:           walletID,
		pubSub:             pubSub,
		direct:             direct,
		threshold:          threshold,
		participantPeerIDs: participantPeerIDs,
		selfPartyID:        selfID,
		partyIDs:           partyIDs,
		outCh:              make(chan tss.Message),
		ErrCh:              make(chan error),
		kvstore:            kvstore,
		keyinfoStore:       keyinfoStore,
		topicComposer: &TopicComposer{
			ComposeBroadcastTopic: func() string {
				return fmt.Sprintf("keygen:broadcast:eddsa:%s", walletID)
			},
			ComposeDirectTopic: func(nodeID string) string {
				return fmt.Sprintf("keygen:direct:eddsa:%s:%s", walletID, nodeID)
			},
		},
		composeKey: func(waleltID string) string {
			return fmt.Sprintf("eddsa:%s", waleltID)
		},
		getRoundFunc: GetEddsaMsgRound,
		successQueue: successQueue,
		sessionType:  SessionTypeEddsa,
	},
		endCh: make(chan *keygen.LocalPartySaveData),
	}
}

func (s *EDDSAKeygenSession) Init() {
	logger.Infof("Initializing session with partyID: %s, peerIDs %s", s.selfPartyID, s.partyIDs)
	ctx := tss.NewPeerContext(s.partyIDs)
	params := tss.NewParameters(tss.Edwards(), ctx, s.selfPartyID, len(s.partyIDs), s.threshold)
	s.party = keygen.NewLocalParty(params, s.outCh, s.endCh)
	logger.Infof("[INITIALIZED] Initialized session successfully partyID: %s, peerIDs %s, walletID %s, threshold = %d", s.selfPartyID, s.partyIDs, s.walletID, s.threshold)
}

func (s *EDDSAKeygenSession) GenerateKey(done func()) {
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

			err = s.kvstore.Put(s.composeKey(s.walletID), keyBytes)
			if err != nil {
				logger.Error("Failed to save key", err, "walletID", s.walletID)
				s.ErrCh <- err
				return
			}

			keyInfo := keyinfo.KeyInfo{
				ParticipantPeerIDs: s.participantPeerIDs,
				Threshold:          s.threshold,
			}

			err = s.keyinfoStore.Save(s.composeKey(s.walletID), &keyInfo)
			if err != nil {
				logger.Error("Failed to save keyinfo", err, "walletID", s.walletID)
				s.ErrCh <- err
				return
			}

			publicKey := saveData.EDDSAPub
			pkX, pkY := publicKey.X(), publicKey.Y()
			pk := edwards.PublicKey{
				Curve: tss.Edwards(),
				X:     pkX,
				Y:     pkY,
			}

			pubKeyBytes := pk.SerializeCompressed()
			s.pubkeyBytes = pubKeyBytes

			err = s.Close()
			if err != nil {
				logger.Error("Failed to close session", err)
			}
			done()
			return

			// solanaAddress := base58.Encode(pubKeyBytes)

			// logger.Info("solana address", "address", solanaAddress)

			// bytes, err := encoding.EncodeEDDSAPubKey(&pk)
			// if err != nil {
			// 	s.ErrCh <- err
			// }

			// k, err := encoding.DecodeEDDSAPubKey(bytes)
			// if err != nil {
			// 	s.ErrCh <- err
			// }

			// x := k.X
			// y := k.Y

			// logger.Info("comparing", "x", x.Cmp(pk.X))
			// logger.Info("comparing", "y", y.Cmp(pk.Y))

			// logger.Info("solana address", "address", solanaAddress)

			// pubKey := &ecdsa.PublicKey{
			// 	Curve: publicKey.Curve(),
			// 	X:     publicKey.X(),
			// 	Y:     publicKey.Y(),
			// }

			// pubKeyBytes, err := encoding.EncodeS256PubKey(pubKey)
			// if err != nil {
			// 	logger.Error("failed to encode public key", err)
			// 	s.ErrCh <- fmt.Errorf("failed to encode public key: %w", err)
			// 	return
			// }

			// successEvent := KeygenSuccessEvent{
			// 	WalletID: s.walletID,
			// 	PubKey:   pubKeyBytes,
			// }

			// successEventBytes, err := json.Marshal(successEvent)
			// if err != nil {
			// 	s.ErrCh <- fmt.Errorf("failed to marshal success event: %w", err)
			// 	return
			// }

			// err = s.successQueue.Enqueue(fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID), successEventBytes, &messaging.EnqueueOptions{
			// 	IdempotententKey: fmt.Sprintf(TypeGenerateWalletSuccess, s.walletID),
			// })
			// if err != nil {
			// 	logger.Error("Failed to publish key generation success message", err)
			// 	s.ErrCh <- fmt.Errorf("Failed to publish key generation success message %w", err)
			// 	return
			// }

			// logger.Info("[COMPLETED KEY GEN] Key generation completed successfully", "walletID", s.walletID)
			// done()
			// return
		}
	}
}
