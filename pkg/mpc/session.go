package mpc

import (
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
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
	ErrCh     chan error
	party     tss.Party
	preParams *keygen.LocalPreParams
	kvstore   kvstore.KVStore
}
