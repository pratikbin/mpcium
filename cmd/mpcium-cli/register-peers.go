package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"syscall"

	"github.com/fystack/mpcium/pkg/config"
	"github.com/fystack/mpcium/pkg/infra"
	"github.com/fystack/mpcium/pkg/logger"
	"github.com/hashicorp/consul/api"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

func registerPeers(ctx context.Context, c *cli.Command) error {
	inputPath := c.String("input")
	environment := c.String("environment")
	authEnabled := c.Bool("auth")

	// Initialize config and logger
	config.InitViperConfig()
	logger.Init(environment, true)

	var authToken string
	if authEnabled {
		var err error
		authToken, err = requestAuthToken()
		if err != nil {
			return err
		}
	}

	// Check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file %s does not exist", inputPath)
	}

	// Read cluster.json file
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read cluster file: %w", err)
	}

	// Parse cluster configuration
	var clusterConfig ClusterConfig
	if err := json.Unmarshal(data, &clusterConfig); err != nil {
		return fmt.Errorf("failed to unmarshal cluster data: %w", err)
	}

	// Validate nodes map
	if len(clusterConfig.Peers) == 0 {
		return fmt.Errorf("no nodes found in the cluster file")
	}

	// Handle cluster ID based on auth status
	if authToken != "" {
		// Use API to register peers and get cluster ID
		newClusterID, err := registerWithAPI(environment, clusterConfig.Peers, authToken)
		if err != nil {
			return err
		}

		// Update the cluster ID in config
		clusterConfig.ClusterID = newClusterID
		// Overwrite the cluster ID in the config file

		// Save updated config back to file
		updatedData, err := json.MarshalIndent(clusterConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal updated cluster config: %w", err)
		}

		if err := os.WriteFile(inputPath, updatedData, 0644); err != nil {
			return fmt.Errorf("failed to write updated cluster.json: %w", err)
		}

		logger.Info("Updated cluster.json with cluster ID from API", "cluster_id", newClusterID)
	} else {
		// Register peers to Consul
		if err := registerPeersToConsul(environment, clusterConfig.Peers, clusterConfig.ClusterID); err != nil {
			return err
		}
	}

	return nil
}

// registerPeersToConsul registers peers to Consul with the given clusterID
func registerPeersToConsul(environment string, nodes map[string]string, clusterID string) error {
	// Base prefix for MPC peers in Consul
	basePrefix := "mpc_peers/"
	// Combine prefix with clusterID
	prefix := basePrefix + clusterID + "/"

	// Connect to Consul
	client := infra.GetConsulClient(environment)
	kv := client.KV()

	// Register peers in Consul
	for nodeName, nodeID := range nodes {
		key := prefix + nodeName
		p := &api.KVPair{Key: key, Value: []byte(nodeID)}

		// Store the key-value pair
		_, err := kv.Put(p, nil)
		if err != nil {
			return fmt.Errorf("failed to store key %s: %w", key, err)
		}
		logger.Info("Registered peer to Consul", "nodeName", nodeName, "nodeID", nodeID)
	}

	logger.Info("Successfully registered peers to Consul", "nodes", nodes, "prefix", prefix)
	return nil
}

func registerWithAPI(environment string, nodes map[string]string, authToken string) (string, error) {
	var apiURL string
	logger.Info("Registering with API", "environment", environment)
	// Determine API URL based on environment
	if environment == "development" || environment == "dev" {
		apiURL = "http://apex.void.exchange/api/v1/workspaces/mpc-clusters/register"
	} else if environment == "production" || environment == "prod" {
		apiURL = "https://api.fystack.io/api/v1/workspaces/mpc-clusters/register"
	} else if environment == "local" {
		apiURL = "http://localhost:8150/api/v1/workspaces/mpc-clusters/register"
	} else {
		return "", fmt.Errorf("unknown environment: %s, expected 'development' or 'production'", environment)
	}

	// Create request payload
	payload := map[string]interface{}{
		"nodes":      nodes,
		"auth_token": authToken,
	}

	// Convert payload to JSON
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for success
	success, ok := result["success"].(bool)
	if !ok || !success {
		message := "unknown error"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return "", fmt.Errorf("API request failed: %s", message)
	}

	// Extract cluster ID
	var clusterID string
	if data, ok := result["data"].(map[string]interface{}); ok {
		clusterID = data["id"].(string)
		clusterName := data["name"].(string)

		logger.Info("Cluster details",
			"clusterID", clusterID,
			"clusterName", clusterName)

		if responseNodes, ok := data["nodes"].(map[string]interface{}); ok {
			nodeInfo := make(map[string]string)
			for nodeName, nodeID := range responseNodes {
				nodeInfo[nodeName] = nodeID.(string)
			}
			logger.Info("Registered nodes", "nodes", nodeInfo)
		}
	}

	if clusterID == "" {
		return "", fmt.Errorf("no cluster ID returned from API")
	}

	logger.Info("Successfully registered with remote API", "clusterID", clusterID)
	return clusterID, nil
}

// requestAuthToken prompts user for authentication token securely
func requestAuthToken() (string, error) {
	fmt.Print("Enter authentication token: ")
	byteToken, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // newline after prompt
	if err != nil {
		return "", fmt.Errorf("failed to read authentication token: %w", err)
	}
	token := string(byteToken)

	if token == "" {
		return "", fmt.Errorf("authentication token cannot be empty")
	}

	return token, nil
}
