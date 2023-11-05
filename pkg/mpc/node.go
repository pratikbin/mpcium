package mpc

import (
	"bytes"
	"fmt"
	"math/big"
	"time"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/cryptoniumX/mpcium/pkg/kvstore"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/cryptoniumX/mpcium/pkg/messaging"
	"github.com/google/uuid"
)

type ID string

type Node struct {
	nodeID  string
	peerIDs []string

	pubSub    messaging.PubSub
	direct    messaging.DirectMessaging
	kvstore   kvstore.KVStore
	preParams *keygen.LocalPreParams

	peerReadyCh chan struct{}
	readyCh     chan struct{}
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
	kvstore kvstore.KVStore,
) *Node {
	peerReadyCh := make(chan struct{}, len(peerIDs)-1)
	for _, peerID := range peerIDs {
		if peerID == nodeID {
			continue
		}
		go func(peerID string) {
			topic := composeReadyTopic(peerID)
			pubSub.Subscribe(topic, func(data []byte) {
				logger.Info("Receive peer ready message", "topic", topic, "peerId", peerID)
				peerReadyCh <- struct{}{}
			})
		}(peerID)

	}

	preParams, err := keygen.GeneratePreParams(1 * time.Minute)
	if err != nil {
		logger.Error("Generate pre params failed", err)
		return nil
	}

	logger.Info("Starting new node, preparams is generated successfully!")

	return &Node{
		nodeID:      nodeID,
		peerIDs:     peerIDs,
		pubSub:      pubSub,
		direct:      direct,
		kvstore:     kvstore,
		preParams:   preParams,
		peerReadyCh: peerReadyCh,
		readyCh:     make(chan struct{}),
	}
}

func (p *Node) ID() string {
	return p.nodeID
}

func composeReadyTopic(nodeID string) string {
	return fmt.Sprintf("%s-%s", nodeID, "ready")
}

func (p *Node) WaitPeersReady() {
	p.pubSub.Publish(composeReadyTopic(p.nodeID), []byte("ready"))

	for i := 0; i < len(p.peerIDs)-1; i++ {
		<-p.peerReadyCh
	}

	for _, peerID := range p.peerIDs {
		logger.Info("Peer status", "peerId", peerID, "status", "ready")
	}

	logger.Info("All peers are ready", "peers", p.peerIDs)
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
		p.preParams,
		p.kvstore,
	)

	return session, nil
}
