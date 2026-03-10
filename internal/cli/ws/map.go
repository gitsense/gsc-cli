/**
 * Component: Workspace Map Command
 * Block-UUID: 5fd67aa2-8d78-4e97-a0ea-46dacef09f1a
 * Parent-UUID: cc7b8cfe-4e41-4383-abbe-0c724dd41d07
 * Version: 1.2.0
 * Description: Implements the 'gsc ws map' command for visualizing and listing workspace blocks across a contract.
 * Language: Go
 * Created-at: 2026-03-10T03:40:02.439Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1), GLM-4.7 (v1.2.0)
 */


package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/contract"
	"github.com/spf13/cobra"
)

var (
	mapAll  bool
	mapList bool
)

// mapCmd represents the 'gsc ws map' command
var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Visualize the workspace hierarchy",
	Long: `Displays a hierarchical view of all workspaces in the current contract.
By default, it provides a focused view of the current workspace.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return handleMap()
	},
}

func init() {
	mapCmd.Flags().BoolVar(&mapAll, "all", false, "Expand all workspaces in the tree")
	mapCmd.Flags().BoolVar(&mapList, "list", false, "Output a flattened list for navigation (fzf)")
}

func handleMap() error {
	// 1. Resolve Environment
	mappedRoot := os.Getenv("GSC_CONTRACT_MAPPED_ROOT")
	if mappedRoot == "" {
		return fmt.Errorf("GSC_CONTRACT_MAPPED_ROOT not set. Are you inside a GitSense workspace?")
	}

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 2. Scan for Workspaces
	entries, err := os.ReadDir(mappedRoot)
	if err != nil {
		return fmt.Errorf("failed to read mapped root: %w", err)
	}

	// 2.5. Resolve Contract Metadata
	// mappedRoot is typically .../dumps/<uuid>/mapped
	// We need the parent directory to get the UUID
	dumpsRoot := filepath.Dir(mappedRoot)
	contractUUID := filepath.Base(dumpsRoot)

	var meta *contract.ContractMetadata
	contractMeta, err := contract.GetContract(contractUUID)
	if err != nil {
		// If we can't load metadata, we'll proceed without the overview section
		// but log a warning for debugging
		fmt.Fprintf(os.Stderr, "Warning: Failed to load contract metadata: %v\n", err)
	} else {
		meta = contractMeta
	}

	var workspaces []contract.ShadowWorkspace
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		manifestPath := filepath.Join(mappedRoot, entry.Name(), "workspace.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue // Skip directories without a manifest
		}

		var ws contract.ShadowWorkspace
		if err := json.Unmarshal(data, &ws); err != nil {
			continue
		}
		workspaces = append(workspaces, ws)
	}

	// Sort workspaces by creation time (newest last)
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].CreatedAt < workspaces[j].CreatedAt
	})

	// 3. Handle Output Modes
	if mapList {
		return renderList(workspaces, mappedRoot, pwd, meta)
	}

	return renderTree(workspaces, mappedRoot, pwd, mapAll, meta)
}

func renderTree(workspaces []contract.ShadowWorkspace, root, pwd string, expandAll bool, meta *contract.ContractMetadata) error {
	// Render Overview Section
	if meta != nil {
		renderOverview(meta, root)
	}

	fmt.Println("Mapped")

	for i, ws := range workspaces {
		isLastWS := i == len(workspaces)-1
		wsPrefix := "├── "
		if isLastWS {
			wsPrefix = "└── "
		}

		wsPath := filepath.Join(root, ws.Hash)
		isCurrentWS := strings.HasPrefix(pwd, wsPath)

		if !isCurrentWS && !expandAll {
			// Collapsed View
			blockLabel := "blocks"
			if len(ws.Files) == 1 {
				blockLabel = "block"
			}
			fmt.Printf("%s%s (%d %s) [msg: %d]\n", wsPrefix, ws.Hash, len(ws.Files), blockLabel, ws.MessageID)
			continue
		}

		// Expanded View
		fmt.Printf("%s%s [msg: %d]\n", wsPrefix, ws.Hash, ws.MessageID)
		
		// Determine child prefix
		childPrefix := "│   "
		if isLastWS {
			childPrefix = "    "
		}

		// We only show unmapped/components and unmapped/snippets for now as per requirements
		renderWorkspaceSubtree(ws, wsPath, pwd, childPrefix)
	}

	return nil
}

func renderWorkspaceSubtree(ws contract.ShadowWorkspace, wsPath, pwd, prefix string) {
	// Group files by their directory type
	components := []contract.MappedFileEntry{}
	snippets := []contract.MappedFileEntry{}

	for _, f := range ws.Files {
		if f.Status == contract.MappedStatusUnmapped {
			if f.Path != "" {
				components = append(components, f)
			} else {
				snippets = append(snippets, f)
			}
		}
	}

	if len(components) > 0 {
		fmt.Printf("%sunmapped\n", prefix)
		fmt.Printf("%s└── components\n", prefix+"    ")
		for i, c := range components {
			marker := "├── "
			if i == len(components)-1 && len(snippets) == 0 {
				marker = "└── "
			}
			
			locMarker := ""
			if filepath.Join(wsPath, "unmapped", "components", c.Path) == pwd {
				locMarker = " *"
			}
			fmt.Printf("%s    %s%s (%s)%s\n", prefix+"    ", marker, c.Path, c.Language, locMarker)
		}
	}

	if len(snippets) > 0 {
		// If we already showed components, the unmapped branch is already open
		if len(components) == 0 {
			fmt.Printf("%sunmapped\n", prefix)
		}
		fmt.Printf("%s└── snippets\n", prefix+"    ")
		for i, s := range snippets {
			marker := "├── "
			if i == len(snippets)-1 {
				marker = "└── "
			}
			
			// Snippets are usually in unmapped/snippets
			locMarker := ""
			if filepath.Join(wsPath, "unmapped", "snippets") == pwd {
				locMarker = " *"
			}
			
			name := fmt.Sprintf("generated_%03d", s.Position+1)
			fmt.Printf("%s    %s%s (%s)%s\n", prefix+"    ", marker, name, s.Language, locMarker)
		}
	}
}

func renderOverview(meta *contract.ContractMetadata, mappedRoot string) {
	fmt.Println("Contract ")
	fmt.Printf("  UUID:   %s\n", meta.UUID)
	if meta.Description != "" {
		fmt.Printf("  Desc:   %s\n", meta.Description)
	}
	fmt.Printf("  Proj:   %s\n", meta.Workdir)
	fmt.Printf("  Dumps:  %s\n", filepath.Dir(mappedRoot))
	fmt.Printf("  Status: %s\n", meta.Status)
	
	// Calculate remaining time for expiration
	remaining := time.Until(meta.ExpiresAt)
	if remaining > 0 {
		fmt.Printf("  Expires in: %s\n", remaining.Round(time.Minute))
	} else {
		fmt.Printf("  Expired: %s\n", meta.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Println()
}

func renderList(workspaces []contract.ShadowWorkspace, root, pwd string, meta *contract.ContractMetadata) error {
	// Note: We accept meta here for consistency, but we don't print it
	// because renderList is typically piped to fzf for navigation,
	// and headers would interfere with the picker.

	for _, ws := range workspaces {
		wsPath := filepath.Join(root, ws.Hash)
		
		for _, f := range ws.Files {
			var targetDir string
			var label string

			if f.Status == contract.MappedStatusMapped {
				targetDir = filepath.Join(wsPath, "mapped", f.Path)
				label = fmt.Sprintf("[%s] mapped/%s (%s)", ws.Hash, f.Path, f.Language)
			} else if f.Path != "" {
				targetDir = filepath.Join(wsPath, "unmapped", "components", f.Path)
				label = fmt.Sprintf("[%s] unmapped/components/%s (%s)", ws.Hash, f.Path, f.Language)
			} else {
				targetDir = filepath.Join(wsPath, "unmapped", "snippets")
				label = fmt.Sprintf("[%s] unmapped/snippets (%s)", ws.Hash, f.Language)
			}

			prefix := "  "
			if targetDir == pwd {
				prefix = "* "
			}

			// Calculate relative path to shorten output
			relPath, err := filepath.Rel(root, targetDir)
			if err != nil {
				relPath = targetDir // Fallback to absolute if rel fails
			}
			fmt.Printf("%s%s | %s\n", prefix, label, relPath)
		}
	}
	return nil
}
