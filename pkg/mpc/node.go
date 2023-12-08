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
	"github.com/hashicorp/consul/api"
)

const (
	PurposeKeygen string = "keygen"
	PurposeSign   string = "sign"
)

type ID string

type Node struct {
	nodeID  string
	peerIDs []string

	pubSub       messaging.PubSub
	direct       messaging.DirectMessaging
	kvstore      kvstore.KVStore
	preParams    *keygen.LocalPreParams
	consulClient *api.Client
	peerReadyCh  chan struct{}
	readyCh      chan struct{}
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
	consulClient *api.Client,
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

	preParams, err := keygen.GeneratePreParams(5 * time.Minute)
	if err != nil {
		logger.Error("Generate pre params failed", err)
		return nil
	}

	logger.Info("Starting new node, preparams is generated successfully!")

	keys, _, err := consulClient.KV().Keys("ready", "", nil)
	if err != nil {
		logger.Error("Get ready keys failed", err)
	}
	if keys != nil {
		// extract nodeID from keys using fmt.Sprintf("ready/%s", nodeID)
		for _, key := range keys {
			logger.Info("Printing keys", "key", key)
			var peerNodeID string
			_, err := fmt.Sscanf(key, "ready/%s", &peerNodeID)
			if err != nil {
				logger.Error("Parse ready key failed", err)
			}

			if nodeID != peerNodeID {
				peerReadyCh <- struct{}{}
			}
		}

	}

	kv := &api.KVPair{
		Key:   ComposeReadyKey(nodeID),
		Value: []byte("ready"),
	}

	_, err = consulClient.KV().Put(kv, nil)
	if err != nil {
		logger.Error("Put ready key failed", err)
	} else {
		logger.Info("Put ready key successfully!", "key", ComposeReadyKey(nodeID))
	}

	return &Node{
		nodeID:       nodeID,
		peerIDs:      peerIDs,
		pubSub:       pubSub,
		direct:       direct,
		kvstore:      kvstore,
		consulClient: consulClient,
		preParams:    preParams,
		peerReadyCh:  peerReadyCh,
		readyCh:      make(chan struct{}),
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

	logger.Info("ALL PEERS ARE READY!", "peers", p.peerIDs)
}

func (p *Node) CreateKeyGenSession(walletID string, threshold int) (*KeygenSession, error) {
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
	logger.Info("Close is being  called")
	_, err := p.consulClient.KV().Delete(ComposeReadyKey(p.nodeID), nil)
	if err != nil {
		logger.Error("Delete ready key failed", err)
	}
}
