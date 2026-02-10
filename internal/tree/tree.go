/**
 * Component: Tree Logic
 * Block-UUID: b633fa98-cf1d-49aa-ab8f-e8a5ef9801a7
 * Parent-UUID: c44f8bfa-ee25-4c3d-b8ce-72d86acc978d
 * Version: 1.0.1
 * Description: Core logic for building, enriching, pruning, and rendering the filesystem tree for gsc tree. Supports CWD-aware construction, metadata enrichment, and coverage reporting.
 * Language: Go
 * Created-at: 2026-02-09T20:55:02.759Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1)
 */


package tree

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yourusername/gsc-cli/internal/search"
)

// Node represents a single entry (file or directory) in the filesystem tree.
type Node struct {
	Name     string                 `json:"name"`
	IsDir    bool                   `json:"type"` // true for directory, false for file
	ChatID   int                    `json:"chat_id,omitempty"`
	Analyzed bool                   `json:"analyzed"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Children []*Node                `json:"children,omitempty"`
}

// TreeStats holds coverage information for the generated tree.
type TreeStats struct {
	TotalFiles    int     `json:"total_files"`
	AnalyzedFiles int     `json:"analyzed_files"`
	Coverage      float64 `json:"coverage_percent"`
}

// BuildTree constructs a hierarchical tree from a list of file paths relative to the repo root.
// It filters the files to only include those within the provided cwdOffset.
func BuildTree(files []string, cwdOffset string) *Node {
	root := &Node{Name: ".", IsDir: true}
	if cwdOffset != "" && cwdOffset != "." {
		root.Name = cwdOffset
	}

	for _, file := range files {
		// Only process files that are within the CWD scope
		if cwdOffset != "" && cwdOffset != "." && !strings.HasPrefix(file, cwdOffset) {
			continue
		}

		// Get path relative to the CWD for tree construction
		relPath := file
		if cwdOffset != "" && cwdOffset != "." {
			var err error
			relPath, err = filepath.Rel(cwdOffset, file)
			if err != nil || strings.HasPrefix(relPath, "..") {
				continue
			}
		}

		if relPath == "." {
			continue
		}

		parts := strings.Split(filepath.ToSlash(relPath), "/")
		current := root
		for i, part := range parts {
			isDir := i < len(parts)-1
			found := false
			for _, child := range current.Children {
				if child.Name == part {
					current = child
					found = true
					break
				}
			}

			if !found {
				newNode := &Node{Name: part, IsDir: isDir}
				current.Children = append(current.Children, newNode)
				current = newNode
			}
		}
	}

	sortTree(root)
	return root
}

// EnrichTree populates the tree nodes with metadata from the provided map.
// The map key should be the full path relative to the repo root.
func EnrichTree(node *Node, currentPath string, metadataMap map[string]search.FileMetadata) {
	// Calculate full path for metadata lookup
	fullPath := node.Name
	if currentPath != "" && currentPath != "." {
		fullPath = filepath.Join(currentPath, node.Name)
	}

	if !node.IsDir {
		if meta, exists := metadataMap[fullPath]; exists {
			node.ChatID = meta.ChatID
			node.Analyzed = meta.ChatID > 0
			node.Metadata = meta.Fields
		}
	}

	for _, child := range node.Children {
		EnrichTree(child, fullPath, metadataMap)
	}
}

// PruneTree removes nodes that have no metadata and no children with metadata.
func PruneTree(node *Node) bool {
	if !node.IsDir {
		return len(node.Metadata) > 0
	}

	var keptChildren []*Node
	for _, child := range node.Children {
		if PruneTree(child) {
			keptChildren = append(keptChildren, child)
		}
	}
	node.Children = keptChildren

	return len(node.Children) > 0
}

// CalculateStats computes coverage statistics for the tree.
func CalculateStats(node *Node) TreeStats {
	total, analyzed := countFiles(node)
	coverage := 0.0
	if total > 0 {
		coverage = (float64(analyzed) / float64(total)) * 100
	}
	return TreeStats{
		TotalFiles:    total,
		AnalyzedFiles: analyzed,
		Coverage:      coverage,
	}
}

// RenderHuman generates the ASCII tree representation.
func RenderHuman(node *Node, indent int, truncate int, fields []string) string {
	var sb strings.Builder
	renderNode(&sb, node, "", true, indent, truncate, fields)
	return sb.String()
}

// RenderJSON generates the JSON representation of the tree and stats.
func RenderJSON(node *Node, stats TreeStats, dbName string, fields []string, pruned bool, cwd string) (string, error) {
	output := map[string]interface{}{
		"version": "1.0.0",
		"context": map[string]interface{}{
			"cwd":       cwd,
			"database":  dbName,
			"fields":    fields,
			"pruned":    pruned,
		},
		"stats": stats,
		"tree":  node,
	}
	bytes, err := json.MarshalIndent(output, "", "  ")
	return string(bytes), err
}

// Internal helpers

func sortTree(node *Node) {
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir // Dirs first
		}
		return node.Children[i].Name < node.Children[j].Name
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

func countFiles(node *Node) (total int, analyzed int) {
	if !node.IsDir {
		total = 1
		if node.Analyzed {
			analyzed = 1
		}
		return
	}
	for _, child := range node.Children {
		t, a := countFiles(child)
		total += t
		analyzed += a
	}
	return
}

func renderNode(sb *strings.Builder, node *Node, prefix string, isLast bool, indentWidth int, truncateLen int, fields []string) {
	if node.Name != "." {
		marker := "├── "
		if isLast {
			marker = "└── "
		}

		// Coverage indicator
		status := "[ ] "
		if !node.IsDir {
			if node.Analyzed {
				status = "[✓] "
			}
		} else {
			status = "" // Don't show status for directories in human mode to keep it clean
		}

		sb.WriteString(prefix + marker + status + node.Name + "\n")

		// Render Metadata Block
		if !node.IsDir && len(node.Metadata) > 0 {
			metaPrefix := prefix
			if isLast {
				metaPrefix += strings.Repeat(" ", indentWidth)
			} else {
				metaPrefix += "│" + strings.Repeat(" ", indentWidth-1)
			}

			for _, field := range fields {
				if val, ok := node.Metadata[field]; ok && val != nil {
					valStr := fmt.Sprintf("%v", val)
					if truncateLen > 0 && len(valStr) > truncateLen {
						valStr = valStr[:truncateLen-3] + "..."
					}

					label := ""
					if len(fields) > 1 {
						label = field + ": "
					}

					sb.WriteString(metaPrefix + "  " + label + valStr + "\n")
				}
			}
		}
	} else {
		sb.WriteString(".\n")
	}

	newPrefix := prefix
	if node.Name != "." {
		if isLast {
			newPrefix += strings.Repeat(" ", indentWidth)
		} else {
			newPrefix += "│" + strings.Repeat(" ", indentWidth-1)
		}
	}

	for i, child := range node.Children {
		renderNode(sb, child, newPrefix, i == len(node.Children)-1, indentWidth, truncateLen, fields)
	}
}
