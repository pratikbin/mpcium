package node

import "github.com/cryptoniumX/mpcium/pkg/messaging"

type ID string

type Party struct {
	id      string
	peerIDs []string

	pubSub messaging.PubSub
	direct messaging.DirectMessaging
}

func NewParty(
	id string,
	peerIDs []string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
) *Party {
	return &Party{
		id:      id,
		peerIDs: peerIDs,
		pubSub:  pubSub,
		direct:  direct,
	}
}
