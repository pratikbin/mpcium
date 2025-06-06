package party

import (
	"context"
	"encoding/json"

	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/logger"
)

type party struct {
	walletID   string
	localParty tss.Party
	partyID    *tss.PartyID
	partyIDs   []*tss.PartyID
	threshold  int
}

type PartyInterface interface {
	PartyID() *tss.PartyID
	GetOutCh() chan tss.Message
	UpdateFromBytes(msgBytes []byte, from *tss.PartyID, isBroadcast bool) (bool, error)
}

func NewParty(walletID string, partyID *tss.PartyID, partyIDs []*tss.PartyID, threshold int) *party {
	return &party{walletID, nil, partyID, partyIDs, threshold}
}

// runParty handles the common party execution loop
func runParty[T any](s PartyInterface, ctx context.Context, party tss.Party, send func(tss.Message), endCh <-chan T, finish func([]byte)) {
	go func() {
		if err := party.Start(); err != nil {
			logger.Error("Failed to start party", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-s.GetOutCh():
			send(msg)
		case result := <-endCh:
			bz, err := json.Marshal(result)
			if err != nil {
				logger.Error("Failed to marshal result", err)
				return
			}
			finish(bz)
			return
		}
	}
}
