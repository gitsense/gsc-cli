/**
 * Component: Brains Help Command
 * Block-UUID: f4a5b6c7-d8e9-0123-fabc-123456789012
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for showing help information about brains commands.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// brainsHelpCmd represents the brains help command
var brainsHelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help for brains commands",
	Long: `Show help information about brains commands.

Brains are local SQLite databases built from Manifests. They store
structured metadata about your repository files that coding agents
can query for context and intelligence.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(`gsc brains - Manage local Brains (SQLite databases)

USAGE
  gsc brains [brain] [flags]
  gsc brains [command]

COMMANDS
  gsc brains              List all active Brains
  gsc brains <name>       Show schema for a specific Brain
  gsc brains delete <db>  Delete a Brain (SQLite database)
  gsc brains help         Show this help message

FLAGS
  --schema    Show schema information for all Brains
  --json      Output rich JSON for coding agents
  --quiet     Suppress headers and hints
  -o format   Output format (table, json)

EXAMPLES
  # List all brains
  gsc brains

  # Show schema for the 'code-intent' brain
  gsc brains code-intent

  # Show schemas for all brains
  gsc brains --schema

  # Delete a brain
  gsc brains delete code-intent

  # Show rich structured output for coding agents
  gsc brains --json

DESCRIPTION
  Brains are local SQLite databases built from Manifests. They store
  structured metadata about your repository files that coding agents
  can query for context and intelligence.

  Use 'gsc manifest import' to build a Brain from a Manifest.
  Use 'gsc query' to find files by metadata.
  Use 'gsc rg' for enriched ripgrep with Brain metadata.`)
		return nil
	},
}

func init() {
	BrainsCmd.AddCommand(brainsHelpCmd)
}
