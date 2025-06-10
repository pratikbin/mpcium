package session

import (
	"context"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/party"
)

type ECDSASession struct {
	*session
}

func NewECDSASession(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int, prepareParams *keygen.LocalPreParams, pubSub messaging.PubSub, direct messaging.DirectMessaging, identityStore identity.Store, kvstore kvstore.KVStore) *ECDSASession {
	s := NewSession(CurveSecp256k1, PurposeKeygen, walletID, pubSub, direct, identityStore, kvstore)
	s.party = party.NewECDSAParty(walletID, partyID, partyIDs, threshold, *prepareParams, nil, s.errCh)
	return &ECDSASession{
		session: s,
	}
}

func (s *ECDSASession) StartKeygen(ctx context.Context, send func(tss.Message), finish func([]byte)) {
	s.party.StartKeygen(ctx, send, finish)
}
