package session

import (
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/party"
)

type EcdsaSession struct {
	*session
}

func NewECDSASession(walletID string, pubSub messaging.PubSub, direct messaging.DirectMessaging, identityStore identity.Store, kvstore kvstore.KVStore) *ECDSASession {
	s := NewSession(CurveSecp256k1, PurposeKeygen, walletID, pubSub, direct, identityStore, kvstore)
	party := party.NewECDSAParty(walletID, s.PartyID(), s.PartyIDs(), s.threshold, s.prepareParams, s.reshareParams, s.saveData)
	s.SetParty(party)
	return &ECDSASession{
		session: s,
		party:   party,
	}
}
