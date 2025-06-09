package session

import "github.com/bnb-chain/tss-lib/v2/tss"

func partyIDToNodeID(partyID *tss.PartyID) string {
	return string(partyID.KeyInt().Bytes())
}
