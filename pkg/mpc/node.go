package mpc

import (
	"bytes"
	"fmt"
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

func PartyIDToNodeID(partyID *tss.PartyID) string {
	return string(partyID.KeyInt().Bytes())
}

func ArePartyIDsEqual(x, y *tss.PartyID) bool {
	return bytes.Equal(x.KeyInt().Bytes(), y.KeyInt().Bytes())

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

func (p *Node) CreateKeyGenSession(walletID string, threshold int) (*Session, error) { // generate pre params
	var selfPartyID *tss.PartyID
	partyIDs := make([]*tss.PartyID, len(p.peerIDs))

	mapPartyIdToNodeId := make(map[string]string)

	for i, peerID := range p.peerIDs {
		if peerID == p.nodeID {
			selfPartyID = CreatePartyID(peerID, "keygen")
			partyIDs[i] = selfPartyID
		} else {

			partyIDs[i] = CreatePartyID(peerID, "keygen")
		}
	}
	fmt.Printf("selfPartyID = %+v\n", selfPartyID)

	sortedPartyIds := tss.SortPartyIDs(partyIDs, 0)

	session := NewSession(
		walletID,
		p.pubSub,
		p.direct,
		selfPartyID,
		sortedPartyIds,
		threshold,
		mapPartyIdToNodeId,
	)

	return session, nil
}
