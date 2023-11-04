package mpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// func TestCreateKeyGenSession(t *testing.T) {
// 	nodeID := uuid.NewString()

// 	peerIDs := []string{
// 		nodeID,
// 		uuid.NewString(),
// 		uuid.NewString(),
// 	}
// 	ctrl := gomock.NewController(t)
// 	defer ctrl.Finish()
// 	pubsub := mock.NewMockPubSub(ctrl)
// 	direct := mock.NewMockDirectMessaging(ctrl)

// 	node := NewNode(nodeID, peerIDs, pubsub, direct)

// 	session, err := node.CreateKeyGenSession()

// 	assert.NoError(t, err)
// 	assert.Len(t, session.PartyIDs(), 3, "Length of partyIDs should be equal")
// 	assert.NotNil(t, session.PartyID())

// 	for i, partyID := range session.PartyIDs() {
// 		// check sortedID
// 		assert.Equal(t, partyID.Index, i, "Index should be equal")
// 	}

// }

func TestPartyIDToNodeID(t *testing.T) {
	partyID := CreatePartyID("4d8cb873-dc86-4776-b6f6-cf5c668f6468", "keygen")
	nodeID := PartyIDToNodeID(partyID)
	assert.Equal(t, nodeID, "4d8cb873-dc86-4776-b6f6-cf5c668f6468", "NodeID should be equal")
}
