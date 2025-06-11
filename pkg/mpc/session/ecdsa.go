package session

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/encoding"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/keyinfo"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/party"
)

type ECDSASession struct {
	*session
}

func NewECDSASession(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int, preParams keygen.LocalPreParams, pubSub messaging.PubSub, direct messaging.DirectMessaging, identityStore identity.Store, kvstore kvstore.KVStore, keyinfoStore keyinfo.Store) *ECDSASession {
	s := NewSession(CurveSecp256k1, PurposeKeygen, walletID, pubSub, direct, identityStore, kvstore, keyinfoStore)
	s.party = party.NewECDSAParty(walletID, partyID, partyIDs, threshold, preParams, nil, s.errCh)
	s.topicComposer = &TopicComposer{
		ComposeBroadcastTopic: func() string {
			return fmt.Sprintf("keygen:broadcast:ecdsa:%s", walletID)
		},
		ComposeDirectTopic: func(nodeID string) string {
			return fmt.Sprintf("keygen:direct:ecdsa:%s:%s", nodeID, walletID)
		},
	}
	s.composeKey = func(walletID string) string {
		return fmt.Sprintf("ecdsa:%s", walletID)
	}
	return &ECDSASession{
		session: s,
	}
}

func (s *ECDSASession) SetSaveData(saveBytes []byte) {
	s.party.SetSaveData(saveBytes)
}

func (s *ECDSASession) StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte)) {
	s.party.StartKeygen(ctx, send, finish)
}

func (s *ECDSASession) StartSigning(ctx context.Context, msg *big.Int, send func(tss.Message), finish func([]byte)) {
	s.party.StartSigning(ctx, msg, send, finish)
}

func (s *ECDSASession) GetPublicKey(data []byte) []byte {
	saveData := &keygen.LocalPartySaveData{}
	err := json.Unmarshal(data, saveData)
	if err != nil {
		return nil
	}

	publicKey := saveData.ECDSAPub
	pubKey := &ecdsa.PublicKey{
		Curve: publicKey.Curve(),
		X:     publicKey.X(),
		Y:     publicKey.Y(),
	}
	pubKeyBytes, err := encoding.EncodeS256PubKey(pubKey)
	if err != nil {
		return nil
	}
	return pubKeyBytes
}
