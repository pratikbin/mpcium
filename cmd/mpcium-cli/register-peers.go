package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fystack/mpcium/pkg/config"
	"github.com/fystack/mpcium/pkg/infra"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli/v3"
)

func registerPeers(ctx context.Context, c *cli.Command) error {
	inputPath := c.String("input")
	environment := c.String("environment")

	// Hardcoded prefix for MPC peers in Consul
	prefix := "mpc_peers/"

	// Check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file %s does not exist", inputPath)
	}

	// Read peers JSON file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse peers data
	peerMap := make(map[string]string)
	if err := json.Unmarshal(data, &peerMap); err != nil {
		return fmt.Errorf("failed to unmarshal JSON data: %w", err)
	}

	if len(peerMap) == 0 {
		return fmt.Errorf("no peers found in the input file")
	}

	// Initialize config and logger
	config.InitViperConfig()
	logger.Init(environment)

	// Connect to Consul
	client := infra.GetConsulClient(environment)
	kv := client.KV()

	// Register peers in Consul
	for nodeName, nodeID := range peerMap {
		key := prefix + nodeName
		p := &api.KVPair{Key: key, Value: []byte(nodeID)}

		// Store the key-value pair
		_, err := kv.Put(p, nil)
		if err != nil {
			return fmt.Errorf("failed to store key %s: %w", key, err)
		}
		fmt.Printf("Registered peer %s with ID %s to Consul\n", nodeName, nodeID)
	}

	logger.Info("Successfully registered peers to Consul", "peers", peerMap, "prefix", prefix)
	return nil
}
