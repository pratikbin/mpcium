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

const (
	PurposeKeygen string = "keygen"
	PurposeSign   string = "sign"
)

type ID string

type Node struct {
	nodeID  string
	peerIDs []string

	pubSub    messaging.PubSub
	direct    messaging.DirectMessaging
	kvstore   kvstore.KVStore
	preParams *keygen.LocalPreParams

	peerRegistry PeerRegistry
}

func CreatePartyID(nodeID string, label string) *tss.PartyID {
	partyID := uuid.NewString()
	key := big.NewInt(0).SetBytes([]byte(nodeID))
	return tss.NewPartyID(partyID, label, key)
}

func PartyIDToNodeID(partyID *tss.PartyID) string {
	return string(partyID.KeyInt().Bytes())
}

func ComparePartyIDs(x, y *tss.PartyID) bool {
	return bytes.Equal(x.KeyInt().Bytes(), y.KeyInt().Bytes())
}

func ComposeReadyKey(nodeID string) string {
	return fmt.Sprintf("ready/%s", nodeID)
}

func NewNode(
	nodeID string,
	peerIDs []string,
	pubSub messaging.PubSub,
	direct messaging.DirectMessaging,
	kvstore kvstore.KVStore,
	peerRegistry PeerRegistry,
) *Node {
	preParams, err := keygen.GeneratePreParams(5 * time.Minute)
	if err != nil {
		logger.Fatal("Generate pre params failed", err)
	}
	logger.Info("Starting new node, preparams is generated successfully!")

	peerRegistry.Ready()
	go peerRegistry.WatchPeersReady()

	return &Node{
		nodeID:       nodeID,
		peerIDs:      peerIDs,
		pubSub:       pubSub,
		direct:       direct,
		kvstore:      kvstore,
		preParams:    preParams,
		peerRegistry: peerRegistry,
	}
}

func (p *Node) ID() string {
	return p.nodeID
}

func composeReadyTopic(nodeID string) string {
	return fmt.Sprintf("%s-%s", nodeID, "ready")
}

func (p *Node) CreateKeyGenSession(walletID string, threshold int) (*KeygenSession, error) {
	if !p.peerRegistry.ArePeersReady() {
		return nil, fmt.Errorf("All peers are not ready!")
	}

	selfPartyID, allPartyIDs := p.generatePartyIDs(PurposeKeygen)
	session := NewKeygenSession(
		walletID,
		p.pubSub,
		p.direct,
		selfPartyID,
		allPartyIDs,
		threshold,
		p.preParams,
		p.kvstore,
	)
	return session, nil
}

func (p *Node) CreateSigningSession(
	walletID string,
	txID string,
	networkInternalCode string,
	threshold int,
) (*SigningSession, error) {
	if !p.peerRegistry.ArePeersReady() {
		return nil, fmt.Errorf("All peers are not ready!")
	}

	selfPartyID, allPartyIDs := p.generatePartyIDs(PurposeKeygen)
	session := NewSigningSession(
		walletID,
		txID,
		networkInternalCode,
		p.pubSub,
		p.direct,
		selfPartyID,
		allPartyIDs,
		threshold,
		p.preParams,
		p.kvstore,
	)

	return session, nil
}

func (p *Node) generatePartyIDs(purpose string) (self *tss.PartyID, all []*tss.PartyID) {
	var selfPartyID *tss.PartyID
	partyIDs := make([]*tss.PartyID, len(p.peerIDs))
	for i, peerID := range p.peerIDs {
		if peerID == p.nodeID {
			selfPartyID = CreatePartyID(peerID, purpose)
			partyIDs[i] = selfPartyID
		} else {
			partyIDs[i] = CreatePartyID(peerID, purpose)
		}
	}
	allPartyIDs := tss.SortPartyIDs(partyIDs, 0)
	return selfPartyID, allPartyIDs
}

func (p *Node) Close() {
	err := p.peerRegistry.Resign()
	if err != nil {
		logger.Error("Resign failed", err)
	}
}
