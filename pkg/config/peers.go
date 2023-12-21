package config

import (
	"fmt"

	"github.com/hashicorp/consul/api"
)

type Peer struct {
	ID   string
	Name string
}

func LoadPeersFromConsul(kv *api.KV, prefix string) ([]Peer, error) {
	// Retrieve node IDs with the "peers" prefix
	pairs, _, err := kv.List(prefix, nil)
	if err != nil {
		return nil, err
	}

	fmt.Println("List of node IDs with the 'peers' prefix:")
	peers := make([]Peer, 0, 10)
	for _, pair := range pairs {
		peers = append(peers, Peer{
			ID: string(pair.Value),
			// remove prefix from key
			Name: pair.Key[len(prefix):],
		})

		fmt.Printf("Key: %s, Value: %s\n", pair.Key, pair.Value)
	}

	return peers, nil
}

func GetNodeID(nodeName string, peers []Peer) string {
	for _, peer := range peers {
		if peer.Name == nodeName {
			return peer.ID
		}
	}

	return ""
}
