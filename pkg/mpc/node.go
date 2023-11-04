package mpc

import (
	"math/big"

	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/google/uuid"
)

type ID string

type Node struct {
	nodeID  string
	peerIDs []string

	pubSub messaging.PubSub
	direct messaging.DirectMessaging
}

func CreatePartyID(nodeID string, label string) *tss.PartyID {
	partyID := uuid.NewString()
	key := big.NewInt(0).SetBytes([]byte(nodeID))
	return tss.NewPartyID(partyID, label, key)
}

func NewNode(
	nodeID string,
	peerIDs []string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
) *Node {
	return &Node{
		nodeID:  nodeID,
		peerIDs: peerIDs,
		pubSub:  pubSub,
		direct:  direct,
	}
}

func (p *Node) ID() string {
	return p.nodeID
}

func (p *Node) CreateKeyGenSession() (*Session, error) { // generate pre params
	var selfPartyID *tss.PartyID
	partyIDs := make([]*tss.PartyID, len(p.peerIDs))

	for i, peerID := range p.peerIDs {
		if peerID == p.nodeID {
			selfPartyID = CreatePartyID(peerID, "keygen")
			partyIDs[i] = selfPartyID
		} else {

			partyIDs[i] = CreatePartyID(peerID, "keygen")
		}
	}

	sortedPartyIds := tss.SortPartyIDs(partyIDs, 0)

	session := &Session{
		selfPartyID: selfPartyID,
		partyIDs:    sortedPartyIds,
	}

	return session, nil
}
