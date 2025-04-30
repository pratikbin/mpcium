package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "mpcium",
		Usage: "Fystack MPC node management tools",
		Commands: []*cli.Command{
			{
				Name:   "generate-peers",
				Usage:  "Generate a new peers.json file",
				Action: generatePeers,
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
						Value:   peersFileName,
					},
				},
			},
			{
				Name:   "register-peers",
				Usage:  "Register peers from a JSON file to Consul",
				Action: registerPeers,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "input",
						Aliases: []string{"i"},
						Usage:   "Input peers JSON file path (default: peers.json)",
						Value:   peersFileName,
					},
					&cli.StringFlag{
						Name:    "environment",
						Aliases: []string{"e"},
						Usage:   "Environment (development, production, etc.)",
						Value:   os.Getenv("ENVIRONMENT"),
					},
				},
			},
			{
				Name:  "generate-identity",
				Usage: "Generate identity files with optional GPG-encrypted private keys for a node",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "node",
						Aliases:  []string{"n"},
						Usage:    "Node name (e.g., node0)",
						Required: true,
					},
					&cli.StringFlag{
						Name:    "peers",
						Aliases: []string{"p"},
						Value:   peersFileName,
						Usage:   "Path to peers.json file",
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
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
