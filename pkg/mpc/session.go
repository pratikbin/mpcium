package mpc

import "github.com/bnb-chain/tss-lib/tss"

type Session struct {
	selfPartyID *tss.PartyID

	// IDs of all parties in the session including self
	partyIDs []*tss.PartyID
}

func (s *Session) PartyID() *tss.PartyID {
	return s.selfPartyID
}

func (s *Session) PartyIDs() []*tss.PartyID {
	return s.partyIDs
}

func (s *Session) Close() {

}
