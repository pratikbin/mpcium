package mpc

import (
	"testing"

	"github.com/cryptoniumX/mpcium/artifacts/mock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCreateKeyGenSession(t *testing.T) {
	nodeID := uuid.NewString()

	peerIDs := []string{
		nodeID,
		uuid.NewString(),
		uuid.NewString(),
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pubsub := mock.NewMockPubSub(ctrl)
	direct := mock.NewMockDirectMessaging(ctrl)

	node := NewNode(nodeID, peerIDs, pubsub, direct)

	session, err := node.CreateKeyGenSession()

	assert.NoError(t, err)
	assert.Len(t, session.PartyIDs(), 3, "Length of partyIDs should be equal")
	assert.NotNil(t, session.PartyID())

	for i, partyID := range session.PartyIDs() {
		// check sortedID
		assert.Equal(t, partyID.Index, i, "Index should be equal")
	}

}
