package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

const (
	// Version information
	VERSION = "0.2.1"
)

func main() {
	cmd := &cli.Command{
		Name:  "mpcium",
		Usage: "Fystack MPC node management tools",
		Commands: []*cli.Command{
			{
				Name:   "generate-cluster",
				Usage:  "Generate a new cluster.json file with cluster ID and peers",
				Action: generateCluster,
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:     "number",
						Aliases:  []string{"n"},
						Usage:    "Number of nodes to generate",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Output file path",
						Value:   "cluster.json",
					},
				},
			},
			{
				Name:   "register-cluster",
				Usage:  "Register cluster from a JSON file to Consul",
				Action: registerPeers,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "input",
						Aliases: []string{"i"},
						Usage:   "Input cluster JSON file path (default: cluster.json)",
						Value:   clusterFileName,
					},
					&cli.StringFlag{
						Name:    "environment",
						Aliases: []string{"e"},
						Usage:   "Environment (development, production, etc.)",
						Value:   os.Getenv("ENVIRONMENT"),
					},
					&cli.BoolFlag{
						Name:    "auth",
						Aliases: []string{"a"},
						Value:   false,
						Usage:   "Authentication token for remote API registration",
					},
				},
			},
			{
				Name:  "generate-identity",
				Usage: "Generate identity files with optional Age-encrypted private keys for a node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "node",
						Aliases:  []string{"n"},
						Usage:    "Node name (e.g., node0)",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "cluster",
						Aliases: []string{"p"},
						Value:   clusterFileName,
						Usage:   "Path to cluster.json file",
					},
					&cli.StringFlag{
						Name:    "output-dir",
						Aliases: []string{"o"},
						Value:   "identity",
						Usage:   "Output directory for identity files",
					},
					&cli.BoolFlag{
						Name:    "encrypt",
						Aliases: []string{"e"},
						Value:   false,
						Usage:   "Encrypt private key with Age (recommended for production)",
					},
					&cli.BoolFlag{
						Name:    "overwrite",
						Aliases: []string{"f"},
						Value:   false,
						Usage:   "Overwrite identity files if they already exist",
					},
				},
				Action: generateIdentity,
			},
			{
				Name:  "generate-initiator",
				Usage: "Generate identity files for an event initiator node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "node-name",
						Aliases: []string{"n"},
						Value:   "event_initiator",
						Usage:   "Name for the initiator node",
					},
					&cli.StringFlag{
						Name:    "output-dir",
						Aliases: []string{"o"},
						Value:   ".",
						Usage:   "Output directory for identity files",
					},
					&cli.BoolFlag{
						Name:    "encrypt",
						Aliases: []string{"e"},
						Value:   false,
						Usage:   "Encrypt private key with Age (recommended for production)",
					},
					&cli.BoolFlag{
						Name:    "overwrite",
						Aliases: []string{"f"},
						Value:   false,
						Usage:   "Overwrite identity files if they already exist",
					},
				},
				Action: generateInitiatorIdentity,
			},
			{
				Name:  "version",
				Usage: "Display detailed version information",
				Action: func(ctx context.Context, c *cli.Command) error {
					fmt.Printf("mpcium-cli version %s\n", VERSION)
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
