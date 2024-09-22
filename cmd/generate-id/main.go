package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/cryptoniumX/mpcium/pkg/config"
	"github.com/cryptoniumX/mpcium/pkg/infra"
	"github.com/cryptoniumX/mpcium/pkg/logger"
	"github.com/google/uuid"
	"github.com/hashicorp/consul/api"
)

var (
	PeerJsonFilePath = "peers.json"
)

func generateUniquePeerID() string {
	// Generate a new UUID
	id, err := uuid.NewRandom()
	if err != nil {
		log.Fatal(err)
	}
	// Convert the UUID to a string representation
	peerID := id.String()
	return peerID
}

func loadPeersFromJSON() ([]string, error) {
	// Initialize an empty peers slice
	var peers []string

	// Check if the JSON file exists
	if _, err := os.Stat("peers.json"); !os.IsNotExist(err) {
		// File exists, load data from the file
		data, err := os.ReadFile(PeerJsonFilePath)
		if err != nil {
			log.Fatalf("Failed to read JSON file: %v", err)
		}

		peerMap := make(map[string]string)
		if len(data) > 0 {
			// JSON data exists in the file, unmarshal it into the peers slice
			if err := json.Unmarshal(data, &peerMap); err != nil {
				log.Fatalf("Failed to unmarshal JSON data: %v", err)
			}
		}

		for _, peer := range peerMap {
			peers = append(peers, peer)
		}

	} else {
		return nil, err
	}

	return peers, nil
}

func loadPeersFromConsul(kv *api.KV, prefix string) ([]string, error) {
	// Retrieve node IDs with the "peers" prefix
	pairs, _, err := kv.List(prefix, nil)
	if err != nil {
		return nil, err

	}

	fmt.Println("Node IDs with the 'peers' prefix:")
	var peers []string
	for _, pair := range pairs {
		peers = append(peers, string(pair.Value))
		fmt.Printf("Key: %s, Value: %s\n", pair.Key, pair.Value)
	}

	return peers, nil
}

func storePeersToJSON(peers any) {
	// Encode the peers to JSON
	peersJSON, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	// Write the JSON data to the output file
	err = os.WriteFile(PeerJsonFilePath, peersJSON, 0644)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Peers data has been written to %s\n", PeerJsonFilePath)
}

func printPeers(peers []string) {
	fmt.Println("Peers:")
	for _, peer := range peers {
		fmt.Println(peer)
	}
}

func main() {
	environment := os.Getenv("ENVIRONMENT")
	config.InitViperConfig(environment)
	logger.Init(environment)
	// Create a new Consul client

	client := infra.GetConsulClient(environment)

	// Create a Key-Value store client
	kv := client.KV()

	// Define the prefix
	prefix := "mpc-peers/"

	peers, err := loadPeersFromConsul(kv, prefix)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Loaded peers from consul:")
	printPeers(peers)

	// 1. Load peers from consul -> if not
	// 2. load peers from json  -> if not
	// 3. generate 3 new peer IDs
	// 4. store peers to consul
	// 5. store peers to json
	// 6. print peers

	if len(peers) == 0 {
		peers, err := loadPeersFromJSON()
		if err != nil {
			fmt.Println(err)
		}

		var nodeIDs []string
		if len(peers) == 0 {
			// Node IDs to store
			nodeIDs = []string{
				generateUniquePeerID(),
				generateUniquePeerID(),
				generateUniquePeerID(),
			}
		} else {
			nodeIDs = peers
		}

		pairs := make(map[string]string)
		for id, nodeID := range nodeIDs {
			key := fmt.Sprintf("%snode%d", prefix, id)
			p := &api.KVPair{Key: key, Value: []byte(nodeID)}

			pairs[fmt.Sprintf("node%d", id)] = nodeID

			// Store the key-value pair
			_, err := kv.Put(p, nil)
			if err != nil {
				log.Printf("Failed to store key %s: %v", key, err)
			} else {
				log.Printf("Stored key %s", key)
			}
		}

		storePeersToJSON(pairs)

	}

	peers, err = loadPeersFromConsul(kv, prefix)
	if err != nil {
		log.Fatal(err)
	}

	printPeers(peers)
}
