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

func (n *Node) CreateKeygenSession(keyType types.KeyType, walletID string, threshold int, successQueue messaging.MessageQueue) (session.Session, error) {
	if n.peerRegistry.GetReadyPeersCount() < int64(threshold+1) {
		return nil, fmt.Errorf("not enough peers to create gen session! expected %d, got %d", threshold+1, n.peerRegistry.GetReadyPeersCount())
	}

	readyPeerIDs := n.peerRegistry.GetReadyPeersIncludeSelf()
	selfPartyID, allPartyIDs := n.generatePartyIDs("keygen", readyPeerIDs)
	switch keyType {
	case types.KeyTypeSecp256k1:
		preparams, err := n.getECDSAPreParams(false)
		if err != nil {
			return nil, fmt.Errorf("failed to get preparams: %w", err)
		}
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
	case types.KeyTypeEd25519:
		eddsaSession := session.NewEDDSASession(
			walletID,
			selfPartyID,
			allPartyIDs,
			threshold,
			n.pubSub,
			n.direct,
			n.identityStore,
			n.kvstore,
			n.keyinfoStore,
		)
		return eddsaSession, nil
	default:
		return nil, fmt.Errorf("invalid key type: %s", keyType)
	}
}

func (n *Node) CreateSigningSession(keyType types.KeyType, walletID string, txID string, threshold int, successQueue messaging.MessageQueue) (session.Session, error) {
	if n.peerRegistry.GetReadyPeersCount() < int64(threshold+1) {
		return nil, fmt.Errorf("not enough peers to create gen session! expected %d, got %d", threshold+1, n.peerRegistry.GetReadyPeersCount())
	}

	readyPeerIDs := n.peerRegistry.GetReadyPeersIncludeSelf()
	selfPartyID, allPartyIDs := n.generatePartyIDs("keygen", readyPeerIDs)
	switch keyType {
	case types.KeyTypeSecp256k1:
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
	case types.KeyTypeEd25519:
		eddsaSession := session.NewEDDSASession(
			walletID,
			selfPartyID,
			allPartyIDs,
			threshold,
			n.pubSub,
			n.direct,
			n.identityStore,
			n.kvstore,
			n.keyinfoStore,
		)

		saveData, err := eddsaSession.GetSaveData()
		if err != nil {
			return nil, fmt.Errorf("failed to get save data: %w", err)
		}

		eddsaSession.SetSaveData(saveData)

		return eddsaSession, nil
	default:
		return nil, fmt.Errorf("invalid key type: %s", keyType)
	}
}

func (n *Node) CreateResharingSession(isOldParty bool, keyType types.KeyType, walletID string, threshold int, successQueue messaging.MessageQueue) (session.Session, error) {
	if n.peerRegistry.GetReadyPeersCount() < int64(threshold+1) {
		return nil, fmt.Errorf("not enough peers to create resharing session! expected %d, got %d", threshold+1, n.peerRegistry.GetReadyPeersCount())
	}
	readyPeerIDs := n.peerRegistry.GetReadyPeersIncludeSelf()
	var selfPartyID *tss.PartyID
	var partyIDs []*tss.PartyID
	if isOldParty {
		selfPartyID, partyIDs = n.generatePartyIDs("keygen", readyPeerIDs)
	} else {
		selfPartyID, partyIDs = n.generatePartyIDs("resharing", readyPeerIDs)
	}

	switch keyType {
	case types.KeyTypeSecp256k1:
		preparams, err := n.getECDSAPreParams(isOldParty)
		if err != nil {
			return nil, fmt.Errorf("failed to get preparams: %w", err)
		}
		ecdsaSession := session.NewECDSASession(walletID, selfPartyID, partyIDs, threshold, *preparams, n.pubSub, n.direct, n.identityStore, n.kvstore, n.keyinfoStore)
		if isOldParty {
			saveData, err := ecdsaSession.GetSaveData()
			if err != nil {
				return nil, fmt.Errorf("failed to get save data: %w", err)
			}
			ecdsaSession.SetSaveData(saveData)
		} else {
			// Initialize new save data for new parties
			saveData := keygen.NewLocalPartySaveData(len(partyIDs))
			saveData.LocalPreParams = *preparams
			saveDataBytes, err := json.Marshal(saveData)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal save data: %w", err)
			}
			ecdsaSession.SetSaveData(saveDataBytes)
		}
		return ecdsaSession, nil
	case types.KeyTypeEd25519:
		eddsaSession := session.NewEDDSASession(walletID, selfPartyID, partyIDs, threshold, n.pubSub, n.direct, n.identityStore, n.kvstore, n.keyinfoStore)
		saveData, err := eddsaSession.GetSaveData()
		if err != nil {
			return nil, fmt.Errorf("failed to get save data: %w", err)
		}
		eddsaSession.SetSaveData(saveData)
		return eddsaSession, nil
	default:
		return nil, fmt.Errorf("invalid key type: %s", keyType)
	}
}

func (n *Node) GetReadyPeersIncludeSelf() []string {
	return n.peerRegistry.GetReadyPeersIncludeSelf()
}

func (n *Node) generatePartyIDs(purpose string, readyPeerIDs []string) (self *tss.PartyID, all []*tss.PartyID) {
	// Pre-allocate slice with exact size needed
	partyIDs := make([]*tss.PartyID, 0, len(readyPeerIDs))

	// Create all party IDs in one pass
	for _, peerID := range readyPeerIDs {
		partyID := createPartyID(peerID, purpose)
		if peerID == n.nodeID {
			self = partyID
		}
		partyIDs = append(partyIDs, partyID)
	}

	// Sort party IDs in place
	all = tss.SortPartyIDs(partyIDs, 0)
	return
}

func (n *Node) getECDSAPreParams(isOldParty bool) (*keygen.LocalPreParams, error) {
	var path string
	if isOldParty {
		path = fmt.Sprintf("preparams.old.%s", n.nodeID)
	} else {
		path = fmt.Sprintf("preparams.%s", n.nodeID)
	}

	preparamsBytes, _ := n.kvstore.Get(path)
	if preparamsBytes == nil {
		logger.Info("Generating preparams", "isOldParty", isOldParty)
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
	logger.Info("Preparams loaded", "isOldParty", isOldParty)
	return &preparams, nil
}

func createPartyID(nodeID string, label string) *tss.PartyID {
	partyID := uuid.NewString()
	key := big.NewInt(0).SetBytes([]byte(nodeID + ":" + label))
	return tss.NewPartyID(partyID, label, key)
}
