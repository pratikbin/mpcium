package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Cluster struct {
	ClusterID string            `json:"cluster_id"`
	Peers     map[string]string `json:"peers"`
}

func LoadClusterFromFile() (Cluster, error) {
	clusterBytes, err := os.ReadFile("cluster.json")
	if err != nil {
		return Cluster{}, fmt.Errorf("failed to read cluster.json: %w", err)
	}

	var cluster Cluster
	if err := json.Unmarshal(clusterBytes, &cluster); err != nil {
		return Cluster{}, fmt.Errorf("failed to parse peers.json: %w", err)
	}

	return cluster, nil
}
