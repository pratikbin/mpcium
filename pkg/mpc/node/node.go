package node

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/bnb-chain/tss-lib/v2/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/fystack/mpcium/pkg/identity"
	"github.com/fystack/mpcium/pkg/keyinfo"
	"github.com/fystack/mpcium/pkg/kvstore"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/fystack/mpcium/pkg/messaging"
	"github.com/fystack/mpcium/pkg/mpc/session"
	"github.com/fystack/mpcium/pkg/types"
	"github.com/google/uuid"
)

type Node struct {
	nodeID  string
	peerIDs []string

	pubSub        messaging.PubSub
	direct        messaging.DirectMessaging
	kvstore       kvstore.KVStore
	keyinfoStore  keyinfo.Store
	identityStore identity.Store

	peerRegistry *registry
}

func NewNode(nodeID string, peerIDs []string, pubSub messaging.PubSub, direct messaging.DirectMessaging, kvstore kvstore.KVStore, keyinfoStore keyinfo.Store, identityStore identity.Store, peerRegistry *registry) *Node {
	go peerRegistry.WatchPeersReady()

	return &Node{
		nodeID:        nodeID,
		peerIDs:       peerIDs,
		pubSub:        pubSub,
		direct:        direct,
		kvstore:       kvstore,
		keyinfoStore:  keyinfoStore,
		identityStore: identityStore,
		peerRegistry:  peerRegistry,
	}
}

func (n *Node) ID() string {
	return n.nodeID
}

func (n *Node) CreateKeygenSession(_ types.KeyType, walletID string, threshold int, successQueue messaging.MessageQueue) (session.Session, error) {
	if n.peerRegistry.GetReadyPeersCount() < int64(threshold+1) {
		return nil, fmt.Errorf("not enough peers to create gen session! expected %d, got %d", threshold+1, n.peerRegistry.GetReadyPeersCount())
	}

	readyPeerIDs := n.peerRegistry.GetReadyPeersIncludeSelf()
	selfPartyID, allPartyIDs := n.generatePartyIDs("keygen", readyPeerIDs)
	preparams, err := n.getECDSAPreParams(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get preparams: %w", err)
	}
	logger.Info("Preparams loaded")

	ecdsaSession := session.NewECDSASession(
		walletID,
		selfPartyID,
		allPartyIDs,
		threshold,
		*preparams,
		n.pubSub,
		n.direct,
		n.identityStore,
		n.kvstore,
		n.keyinfoStore,
	)

	return ecdsaSession, nil
}

func (n *Node) CreateSigningSession(_ types.KeyType, walletID string, txID string, threshold int, successQueue messaging.MessageQueue) (session.Session, error) {
	if n.peerRegistry.GetReadyPeersCount() < int64(threshold+1) {
		return nil, fmt.Errorf("not enough peers to create gen session! expected %d, got %d", threshold+1, n.peerRegistry.GetReadyPeersCount())
	}

	readyPeerIDs := n.peerRegistry.GetReadyPeersIncludeSelf()
	selfPartyID, allPartyIDs := n.generatePartyIDs("keygen", readyPeerIDs)
	ecdsaSession := session.NewECDSASession(
		walletID,
		selfPartyID,
		allPartyIDs,
		threshold,
		keygen.LocalPreParams{},
		n.pubSub,
		n.direct,
		n.identityStore,
		n.kvstore,
		n.keyinfoStore,
	)
	saveData, err := ecdsaSession.GetSaveData()
	if err != nil {
		return nil, fmt.Errorf("failed to get save data: %w", err)
	}

	ecdsaSession.SetSaveData(saveData)

	return ecdsaSession, nil
}

func (n *Node) GetReadyPeersIncludeSelf() []string {
	return n.peerRegistry.GetReadyPeersIncludeSelf()
}

func (n *Node) generatePartyIDs(purpose string, readyPeerIDs []string) (self *tss.PartyID, all []*tss.PartyID) {
	var selfPartyID *tss.PartyID
	partyIDs := make([]*tss.PartyID, len(readyPeerIDs))
	for i, peerID := range readyPeerIDs {
		if peerID == n.nodeID {
			selfPartyID = createPartyID(peerID, purpose)
			partyIDs[i] = selfPartyID
		} else {
			partyIDs[i] = createPartyID(peerID, purpose)
		}
	}
	allPartyIDs := tss.SortPartyIDs(partyIDs, 0)
	return selfPartyID, allPartyIDs
}

func (n *Node) getECDSAPreParams(isOldParty bool) (*keygen.LocalPreParams, error) {
	var path string
	if isOldParty {
		path = fmt.Sprintf("preparams.old.%s", n.nodeID)
	} else {
		path = fmt.Sprintf("preparams.%s", n.nodeID)
	}

	preparamsBytes, _ := n.kvstore.Get(path)
	// if err != nil {
	// 	return nil, err
	// }

	if preparamsBytes == nil {
		preparams, err := keygen.GeneratePreParams(5 * time.Minute)
		if err != nil {
			return nil, err
		}
		preparamsBytes, err = json.Marshal(preparams)
		if err != nil {
			return nil, err
		}
		n.kvstore.Put(path, preparamsBytes)
		return preparams, nil
	}

	var preparams keygen.LocalPreParams
	if err := json.Unmarshal(preparamsBytes, &preparams); err != nil {
		return nil, err
	}
	return &preparams, nil
}

func createPartyID(nodeID string, label string) *tss.PartyID {
	partyID := uuid.NewString()
	key := big.NewInt(0).SetBytes([]byte(nodeID + ":" + label))
	return tss.NewPartyID(partyID, label, key)
}
