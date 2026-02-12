/**
 * Component: Tree Logic
 * Block-UUID: b173275f-24c3-4aa2-b437-cea29e24b901
 * Parent-UUID: 8c8d1c1a-e9b0-45d5-b768-d64894efac9a
 * Version: 1.2.1
 * Description: Enhanced tree logic to support semantic filtering and structural focus. Added 'Matched' and 'Visible' states to Node to enable the "Semantic Heat Map" visualization. Updated rendering to support name-hiding for non-matching files and multi-path focus pruning.
 * Language: Go
 * Created-at: 2026-02-12T04:22:17.605Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.0.1), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.2.1)
 */


package tree

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/yourusername/gsc-cli/internal/search"
)

// Node represents a single entry (file or directory) in the filesystem tree.
type Node struct {
	Name     string                 `json:"name"`
	IsDir    bool                   `json:"is_dir"` // true for directory, false for file
	ChatID   int                    `json:"chat_id,omitempty"`
	Analyzed bool                   `json:"analyzed"`
	Matched  bool                   `json:"matched"` // true if it satisfies the semantic filter
	Visible  bool                   `json:"visible"` // true if it or any descendant is matched
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Children []*Node                `json:"children,omitempty"`
}

// TreeStats holds coverage information for the generated tree.
type TreeStats struct {
	TotalFiles    int     `json:"total_files"`
	AnalyzedFiles int     `json:"analyzed_files"`
	MatchedFiles  int     `json:"matched_files"`
	Coverage      float64 `json:"coverage_percent"`
}

// PortableNode represents a simplified node for the ai-portable format.
type PortableNode struct {
	Name     string                 `json:"name"`
	IsDir    bool                   `json:"is_dir"`
	Matched  bool                   `json:"matched"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Children []*PortableNode        `json:"children,omitempty"`
}

// BuildTree constructs a hierarchical tree from a list of file paths relative to the repo root.
// It filters the files to only include those within the provided cwdOffset and focusPatterns.
func BuildTree(files []string, cwdOffset string, focusPatterns []string) *Node {
	root := &Node{Name: ".", IsDir: true, Visible: true}
	if cwdOffset != "" && cwdOffset != "." {
		root.Name = cwdOffset
	}

	for _, file := range files {
		// 1. Structural Filter: Check CWD scope
		if cwdOffset != "" && cwdOffset != "." && !strings.HasPrefix(file, cwdOffset) {
			continue
		}

		// 2. Structural Filter: Check Focus Patterns
		if len(focusPatterns) > 0 {
			inFocus := false
			for _, pattern := range focusPatterns {
				if ok, _ := doublestar.Match(pattern, file); ok {
					inFocus = true
					break
				}
			}
			if !inFocus {
				continue
			}
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
				newNode := &Node{Name: part, IsDir: isDir, Visible: true}
				current.Children = append(current.Children, newNode)
				current = newNode
			}
		}
	}

	sortTree(root)
	return root
}

// EnrichTree populates the tree nodes with metadata and evaluates semantic filters.
func EnrichTree(node *Node, currentPath string, metadataMap map[string]search.FileMetadata, filters []search.FilterCondition) {
	fullPath := node.Name
	if currentPath != "" && currentPath != "." {
		fullPath = filepath.Join(currentPath, node.Name)
	}

	if !node.IsDir {
		if meta, exists := metadataMap[fullPath]; exists {
			node.ChatID = meta.ChatID
			node.Analyzed = meta.ChatID > 0
			node.Metadata = meta.Fields

			// Evaluate Semantic Filter
			if len(filters) > 0 {
				node.Matched = search.CheckFilters(node.Metadata, filters)
			} else {
				node.Matched = true // If no filter, everything is a match
			}
		} else {
			// If file is not in metadata map, it can't match a metadata filter
			node.Matched = len(filters) == 0
		}
		node.Visible = node.Matched
	}

	for _, child := range node.Children {
		EnrichTree(child, fullPath, metadataMap, filters)
	}
}

// CalculateVisibility updates the Visible flag for directories based on their children.
// A directory is visible if it is matched or has any visible descendant.
func CalculateVisibility(node *Node) bool {
	if !node.IsDir {
		return node.Visible
	}

	anyChildVisible := false
	for _, child := range node.Children {
		if CalculateVisibility(child) {
			anyChildVisible = true
		}
	}

	node.Visible = anyChildVisible
	return node.Visible
}

// PruneTree removes nodes that are not marked as Visible.
func PruneTree(node *Node) bool {
	if !node.IsDir {
		return node.Visible
	}

	var keptChildren []*Node
	for _, child := range node.Children {
		if PruneTree(child) {
			keptChildren = append(keptChildren, child)
		}
	}
	node.Children = keptChildren

	return len(node.Children) > 0 || node.Matched
}

// CalculateStats computes coverage and match statistics for the tree.
func CalculateStats(node *Node) TreeStats {
	total, analyzed, matched := countFiles(node)
	coverage := 0.0
	if total > 0 {
		coverage = (float64(analyzed) / float64(total)) * 100
	}
	return TreeStats{
		TotalFiles:    total,
		AnalyzedFiles: analyzed,
		MatchedFiles:  matched,
		Coverage:      coverage,
	}
}

// RenderHuman generates the ASCII tree representation with Heat Map support.
func RenderHuman(node *Node, indent int, truncate int, fields []string, noCompact bool) string {
	var sb strings.Builder
	renderNode(&sb, node, "", true, indent, truncate, fields, noCompact)
	return sb.String()
}

// RenderJSON generates the JSON representation of the tree and stats.
func RenderJSON(node *Node, stats TreeStats, dbName string, fields []string, filters []search.FilterCondition, focus []string, pruned bool, cwd string) (string, error) {
	output := map[string]interface{}{
		"version": "1.1.0",
		"context": map[string]interface{}{
			"cwd":       cwd,
			"database":  dbName,
			"fields":    fields,
			"filters":   filters,
			"focus":     focus,
			"pruned":    pruned,
		},
		"stats": stats,
		"tree":  node,
	}
	bytes, err := json.MarshalIndent(output, "", "  ")
	return string(bytes), err
}

// RenderPortableJSON generates the AI-Portable JSON representation.
func RenderPortableJSON(node *Node, stats TreeStats, fields []string, pruned bool, cwd string) (string, error) {
	portableTree := convertToPortableNode(node)

	output := map[string]interface{}{
		"context": map[string]interface{}{
			"about": "This JSON represents a hierarchical Git tree. Each node represents a file or directory. Metadata is included for files where available to provide additional context for analysis.",
			"cwd":   cwd,
			"fields": fields,
			"pruned": pruned,
		},
		"stats": map[string]interface{}{
			"total_files":               stats.TotalFiles,
			"files_with_metadata":       stats.AnalyzedFiles,
			"matched_files":             stats.MatchedFiles,
			"metadata_coverage_percent": stats.Coverage,
		},
		"tree": portableTree,
	}

	bytes, err := json.MarshalIndent(output, "", "  ")
	return string(bytes), err
}

// convertToPortableNode recursively converts a standard Node to a PortableNode.
func convertToPortableNode(node *Node) *PortableNode {
	if node == nil {
		return nil
	}
	pn := &PortableNode{
		Name:     node.Name,
		IsDir:    node.IsDir,
		Matched:  node.Matched,
		Metadata: node.Metadata,
	}
	for _, child := range node.Children {
		pn.Children = append(pn.Children, convertToPortableNode(child))
	}
	return pn
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

func countFiles(node *Node) (total int, analyzed int, matched int) {
	if !node.IsDir {
		total = 1
		if node.Analyzed {
			analyzed = 1
		}
		if node.Matched {
			matched = 1
		}
		return
	}
	for _, child := range node.Children {
		t, a, m := countFiles(child)
		total += t
		analyzed += a
		matched += m
	}
	return
}

func renderNode(sb *strings.Builder, node *Node, prefix string, isLast bool, indentWidth int, truncateLen int, fields []string, noCompact bool) {
	if node.Name != "." {
		marker := "├── "
		if isLast {
			marker = "└── "
		}

		// Status Indicator
		status := ""
		if !node.IsDir {
			if node.Matched {
				status = "[✓] "
			} else {
				status = "[○] "
			}
		}

		// Name Hiding (Heat Map Logic)
		displayName := node.Name
		if !node.IsDir && !node.Matched && !noCompact {
			displayName = ""
		}

		sb.WriteString(prefix + marker + status + displayName + "\n")

		// Render Metadata Block (Only for matches)
		if !node.IsDir && node.Matched && len(node.Metadata) > 0 {
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
		// Render if visible, OR if noCompact is true (show full context)
		if child.Visible || noCompact {
			renderNode(sb, child, newPrefix, i == len(node.Children)-1, indentWidth, truncateLen, fields, noCompact)
		}
	}
}
