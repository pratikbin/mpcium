package mpc

import (
	"fmt"
	"sync"
	"time"

	"github.com/cryptoniumX/mpcium/pkg/infra"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/hashicorp/consul/api"
	"github.com/samber/lo"
)

const (
	ReadinessCheckPeriod = 5 * time.Second
)

type PeerRegistry interface {
	Ready() error
	ArePeersReady() bool
	WatchPeersReady()
	// Resign is called by the node when it is going to shutdown
	Resign() error
}

type registry struct {
	nodeID      string
	peerNodeIDs []string
	readyMap    map[string]bool

	mu    sync.RWMutex
	ready bool // ready is true when all peers are ready

	consulKV infra.ConsulKV
}

func NewRegistry(
	nodeID string,
	peerNodeIDs []string,
	consulKV infra.ConsulKV,
) *registry {
	return &registry{
		consulKV:    consulKV,
		nodeID:      nodeID,
		peerNodeIDs: getPeerIDsExceptSelf(nodeID, peerNodeIDs),
		readyMap:    make(map[string]bool),
	}
}

func getPeerIDsExceptSelf(nodeID string, peerNodeIDs []string) []string {
	peerIDs := make([]string, 0, len(peerNodeIDs))
	for _, peerID := range peerNodeIDs {
		if peerID != nodeID {
			peerIDs = append(peerIDs, peerID)
		}
	}
	return peerIDs
}

func (r *registry) readyKey(nodeID string) string {
	return fmt.Sprintf("ready/%s", nodeID)
}

func (r *registry) registerReadyPairs(peerIDs []string) {
	for _, peerID := range peerIDs {
		ready, exist := r.readyMap[peerID]
		if !exist {
			r.readyMap[peerID] = true
			logger.Info("Register", "peerID", peerID)
		} else if !ready {
			r.readyMap[peerID] = true
			logger.Info("Reconnecting...", "peerID", peerID)
		}
	}

	if len(peerIDs) == len(r.peerNodeIDs) && !r.ready {
		r.mu.Lock()
		r.ready = true
		r.mu.Unlock()
		logger.Info("ALL PEERS ARE READY! Starting to accept MPC requests")
	}

}

// Ready is called by the node when it complete generate preparams and starting to accept
// incoming requests
func (r *registry) Ready() error {
	k := r.readyKey(r.nodeID)

	kv := &api.KVPair{
		Key:   k,
		Value: []byte("true"),
	}

	_, err := r.consulKV.Put(kv, nil)
	if err != nil {
		return fmt.Errorf("Put ready key failed: %w", err)
	}

	return nil
}

func (r *registry) WatchPeersReady() {
	ticker := time.NewTicker(ReadinessCheckPeriod)
	// first tick is executed immediately
	for ; true; <-ticker.C {
		pairs, _, err := r.consulKV.List("ready/", nil)
		if err != nil {
			logger.Error("List ready keys failed", err)
		}

		newReadyPeerIDs := r.getReadyPeers(pairs)
		if len(newReadyPeerIDs) != len(r.peerNodeIDs) {
			logger.Info("Peers are not ready yet", "ready", len(pairs), "expected", len(r.peerNodeIDs)+1)
			r.mu.Lock()
			r.ready = false
			r.mu.Unlock()

			var readyPeerIDs []string
			for peerID, isReady := range r.readyMap {
				if isReady {
					readyPeerIDs = append(readyPeerIDs, peerID)
				}
			}

			disconnecteds, _ := lo.Difference(readyPeerIDs, newReadyPeerIDs)
			if len(disconnecteds) > 0 {
				for _, peerID := range disconnecteds {
					logger.Warn("Peer disconnected!", "peerID", peerID)
					r.readyMap[peerID] = false
				}

			}

		}
		r.registerReadyPairs(newReadyPeerIDs)
	}
}

func (r *registry) getReadyPeers(kvPairs api.KVPairs) []string {
	var peers []string
	for _, k := range kvPairs {
		var peerNodeID string
		_, err := fmt.Sscanf(k.Key, "ready/%s", &peerNodeID)
		if err != nil {
			logger.Error("Parse ready key failed", err)
		}
		if peerNodeID == r.nodeID {
			continue
		}

		peers = append(peers, peerNodeID)
	}

	return peers
}

func (r *registry) ArePeersReady() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.ready
}

func (r *registry) Resign() error {
	k := r.readyKey(r.nodeID)

	_, err := r.consulKV.Delete(k, nil)
	if err != nil {
		return fmt.Errorf("Delete ready key failed: %w", err)
	}

	return nil
}
