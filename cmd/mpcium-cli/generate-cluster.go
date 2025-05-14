package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"
)

// ClusterConfig structure for cluster.json
type ClusterConfig struct {
	ClusterID string            `json:"cluster_id"`
	Peers     map[string]string `json:"peers"`
}

func generateCluster(ctx context.Context, c *cli.Command) error {
	numNodes := c.Int("number")
	if numNodes < 1 {
		return fmt.Errorf("number of nodes must be at least 1")
	}

	outputPath := c.String("output")

	// Check if file already exists
	if _, err := os.Stat(outputPath); err == nil {
		return fmt.Errorf("file %s already exists, won't overwrite", outputPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking file status: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Generate cluster ID
	clusterID, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to generate cluster UUID: %w", err)
	}

	// Generate peers data
	peers := make(map[string]string)
	for i := 0; i < numNodes; i++ {
		nodeName := fmt.Sprintf("node%d", i)
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to generate node UUID: %w", err)
		}
		peers[nodeName] = id.String()
	}

	// Create cluster config
	clusterConfig := ClusterConfig{
		ClusterID: clusterID.String(),
		Peers:     peers,
	}

	// Convert to JSON
	clusterJSON, err := json.MarshalIndent(clusterConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(outputPath, clusterJSON, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Successfully generated cluster file at %s with %d nodes\n", outputPath, numNodes)
	return nil
}
