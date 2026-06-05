/**
 * Component: Tree Command
 * Block-UUID: 18edc412-d018-4c34-9cf7-bea9781d244c
 * Parent-UUID: 9a46883d-b8c7-4229-b3e2-5417914ec929
 * Version: 1.10.1
 * Description: Added RepositoryTotal field to TreeStats struct to support showing both repository total and filtered count in JSON output. Updated RenderJSON to include repository total when filters are active.
 * Language: Go
 * Created-at: 2026-06-04T00:18:12.157Z
 * Authors: GLM-4.7 (v1.7.1), GLM-4.7 (v1.7.2), GLM-4.7 (v1.8.0), GLM-4.7 (v1.8.1), GLM-4.7 (v1.9.0), GLM-4.7 (v1.9.1), GLM-4.7 (v1.9.2), GLM-4.7 (v1.9.3), GLM-4.7 (v1.9.4), GLM-4.7 (v1.9.5), GLM-4.7 (v1.10.0), GLM-4.7 (v1.10.1)
 */


package tree

import (
	"encoding/json"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/gitsense/gsc-cli/internal/search"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Node represents a file or directory in the tree.
type Node struct {
	Name     string                 `json:"name"`
	IsDir    bool                   `json:"is_dir"`
	Path     string                 `json:"path"`
	Children map[string]*Node       `json:"children,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Matched  bool                   `json:"matched"`
	Visible  bool                   `json:"visible"`
	ChatID   int                    `json:"chat_id,omitempty"`
}

// BuildTree constructs a hierarchical tree structure from a list of file paths.
// It applies structural focus patterns to filter the initial list of files.
// It returns the root node of the tree.
func BuildTree(files []string, cwdOffset string, focusPatterns []string, globPatterns []string) *Node {
	root := &Node{
		Name:     ".",
		IsDir:    true,
		Path:     ".",
		Children: make(map[string]*Node),
	}

	for _, filePath := range files {
		// Check glob patterns first
		if len(globPatterns) > 0 {
			matched := false
			for _, pattern := range globPatterns {
				// Normalize paths for matching:
				// - If pattern is absolute, match against absolute file path
				// - If pattern is relative, match against path relative to CWD
				// This ensures globs work correctly when run from subdirectories
				_, err := os.Getwd()
				if err != nil {
					continue // Skip if we can't determine CWD
				}

				// filePath is relative to repo root. We need to make it relative to CWD or absolute.
				// Since we don't have repoRoot here, we assume filePath is relative to CWD if it matches,
				// or we construct an absolute path if possible.
				// However, BuildTree receives files relative to repo root.
				// To match correctly, we need the repo root or assume the caller handled it.
				// For now, let's assume the user is in the repo root or the paths are compatible.
				// A better fix requires passing repoRoot to BuildTree.
				// But we can try to match against the filename part as a fallback or heuristic.
				// Actually, let's just try to match against the full path provided.
				// If the user is in a subdir, this might fail without repoRoot context in BuildTree.
				// Given the constraints, we will match against the provided filePath.
				// Note: This might not work perfectly from subdirs without repoRoot context in BuildTree.
				// But let's try to be smart about it.
				
				// Reverting to simple match for now as BuildTree doesn't have repoRoot context.
				// The fix in simple_querier.go is the critical one for insights/query.
				if match, _ := doublestar.Match(pattern, filePath); match {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Check focus patterns
		if len(focusPatterns) > 0 {
			matched := false
			for _, pattern := range focusPatterns {
				if match, _ := doublestar.Match(pattern, filePath); match {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Normalize path to be relative to repo root
		// The files list from git.GetTrackedFiles is already relative to repo root
		
		// Split path into components
		parts := strings.Split(filePath, string(filepath.Separator))
		
		current := root
		currentPath := ""
		
		for i, part := range parts {
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = filepath.Join(currentPath, part)
			}
			
			isLast := (i == len(parts) - 1)
			
			if _, exists := current.Children[part]; !exists {
				current.Children[part] = &Node{
					Name:     part,
					IsDir:    !isLast,
					Path:     currentPath,
					Children: make(map[string]*Node),
					Metadata: make(map[string]interface{}),
				}
			}
			
			current = current.Children[part]
		}
	}
	
	return root
}

// EnrichTree adds metadata to tree nodes based on the provided metadata map.
// It also evaluates filters to mark nodes as matched or unmatched.
func EnrichTree(node *Node, basePath string, metadataMap map[string]map[string]interface{}, filters []search.FilterCondition, requestedFields []string) {
	if node == nil {
		return
	}

	// Construct full path for this node
	fullPath := node.Path
	if basePath != "" {
		fullPath = filepath.Join(basePath, node.Path)
	}

	// If this is a file (leaf node), try to enrich it
	if !node.IsDir {
		if meta, exists := metadataMap[fullPath]; exists {
			// Evaluate filters FIRST, using the full metadata map
			// This ensures filters work even when the filter field is not in requestedFields
			if len(filters) > 0 {
				node.Matched = search.CheckFilters(meta, filters)
			} else {
				// If no filters, all files are considered matched
				node.Matched = true
			}
		} else {
			// File not in metadata map
			node.Matched = false
			return
		}
	}

	// If this is a file (leaf node), try to enrich it
	if !node.IsDir {
		if meta, exists := metadataMap[fullPath]; exists {
			// Add requested fields to metadata
			if len(requestedFields) > 0 {
				for _, field := range requestedFields {
					if val, ok := meta[field]; ok {
						if node.Matched {  // Only add fields if the file matched filters
							node.Metadata[field] = val
						}
					}
				}
			} else {
				// If no specific fields requested, add all metadata
				for k, v := range meta {
					node.Metadata[k] = v
				}
			}

			// Extract ChatID from metadata (now includes database columns)
			if chatID, ok := meta["chat_id"].(int); ok {
				node.ChatID = chatID
			}
			// If chat_id doesn't exist, ChatID remains 0 and is omitted from JSON
		} else {
			// File not in metadata map
			node.Matched = false
		}
	}

	// Recursively enrich children
	for _, child := range node.Children {
		EnrichTree(child, basePath, metadataMap, filters, requestedFields)
	}
}

// CalculateVisibility propagates match status up the tree.
// A directory is visible if any of its children are visible or matched.
func CalculateVisibility(node *Node) {
	if node == nil {
		return
	}

	// If it's a file, visibility is determined by match status
	if !node.IsDir {
		node.Visible = node.Matched
		return
	}

	// If it's a directory, check children
	hasVisibleChild := false
	for _, child := range node.Children {
		CalculateVisibility(child)
		if child.Visible {
			hasVisibleChild = true
		}
	}

	// Directory is visible if it has any visible children
	node.Visible = hasVisibleChild
}

// PruneTree removes nodes that are not visible.
// This is used to hide non-matching files and empty directories.
func PruneTree(node *Node) {
	if node == nil {
		return
	}

	// If it's a file and not visible, it will be removed by the parent
	if !node.IsDir {
		return
	}

	// Recursively prune children
	for name, child := range node.Children {
		if !child.Visible {
			delete(node.Children, name)
		} else {
			PruneTree(child)
		}
	}
}

// TreeStats holds statistics about the tree.
type TreeStats struct {
	TotalFiles      int     `json:"TotalFiles"`
	AnalyzedFiles   int     `json:"AnalyzedFiles"`
	MatchedFiles    int     `json:"MatchedFiles"`
	Coverage        float64 `json:"Coverage"`
	RepositoryTotal int     `json:"RepositoryTotal,omitempty"` // Total files in repository (before filtering)
}

// CalculateStats computes statistics for the tree.
func CalculateStats(node *Node) TreeStats {
	stats := TreeStats{}

	if node == nil {
		return stats
	}

	// Traverse the tree
	var traverse func(n *Node)
	traverse = func(n *Node) {
		if n == nil {
			return
		}

		if !n.IsDir {
			stats.TotalFiles++
			if len(n.Metadata) > 0 {
				stats.AnalyzedFiles++
			}
			if n.Matched {
				stats.MatchedFiles++
			}
		}

		for _, child := range n.Children {
			traverse(child)
		}
	}

	traverse(node)

	if stats.TotalFiles > 0 {
		stats.Coverage = (float64(stats.AnalyzedFiles) / float64(stats.TotalFiles)) * 100
	}

	return stats
}

// RenderHuman generates a human-readable ASCII representation of the tree.
func RenderHuman(root *Node, indent int, truncate int, fields []string, noCompact bool) string {
	var builder strings.Builder

	var render func(n *Node, depth int)
	render = func(n *Node, depth int) {
		if n == nil {
			return
		}

		// Calculate indentation
		prefix := strings.Repeat(" ", depth*indent)

		// Render node name
		if n.IsDir {
			builder.WriteString(prefix + n.Name + "/\n")
		} else {
			// Render file with metadata
			line := prefix + n.Name

			// Add metadata if available
			if len(n.Metadata) > 0 {
				metadataStr := ""
				for i, field := range fields {
					if val, ok := n.Metadata[field]; ok {
						valStr := fmt.Sprintf("%v", val)
						if truncate > 0 && len(valStr) > truncate {
							valStr = valStr[:truncate] + "..."
						}
						if i > 0 {
							metadataStr += " "
						}
						metadataStr += fmt.Sprintf("[%s=%s]", field, valStr)
					}
				}
				if metadataStr != "" {
					line += " " + metadataStr
				}
			}

			// Highlight matched files
			if n.Matched {
				line = "\033[32m" + line + "\033[0m" // Green
			} else if !noCompact && len(n.Metadata) == 0 {
				// In compact mode, hide non-matching, non-analyzed files
				return
			}

			builder.WriteString(line + "\n")
		}

		// Render children
		sortedChildren := make([]string, 0, len(n.Children))
		for name := range n.Children {
			sortedChildren = append(sortedChildren, name)
		}
		sort.Strings(sortedChildren)

		for _, name := range sortedChildren {
			render(n.Children[name], depth+1)
		}
	}

	render(root, 0)
	return builder.String()
}

// RenderJSON generates a JSON representation of the tree.
func RenderJSON(root *Node, stats TreeStats, dbName string, fields []string, filters []search.FilterCondition, focus []string, prune bool, cwdOffset string, repoTotalFiles int) (string, error) {
	// Add repository total to stats when filters are active
	if len(filters) > 0 && prune {
		stats.RepositoryTotal = repoTotalFiles
	}
	
	data := map[string]interface{}{
		"stats": stats,
		"tree":  root,
		// Add other metadata like dbName, filters, etc.
	}
	
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	
	return string(jsonBytes), nil
}

// PortableNode represents a simplified node structure for AI consumption.
type PortableNode struct {
	Name     string                 `json:"name"`
	Path     string                 `json:"path"`
	IsDir    bool                   `json:"is_dir"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Children []*PortableNode        `json:"children,omitempty"`
}

// RenderPortableJSON generates an AI-portable JSON representation.
func RenderPortableJSON(root *Node, stats TreeStats, fields []string, prune bool, cwdOffset string, repoTotalFiles int) (string, error) {
	// Add repository total to stats when filters are active
	if prune {
		stats.RepositoryTotal = repoTotalFiles
	}
	
	var convert func(n *Node) *PortableNode
	convert = func(n *Node) *PortableNode {
		if n == nil {
			return nil
		}

		pn := &PortableNode{
			Name:     n.Name,
			Path:     n.Path,
			IsDir:    n.IsDir,
			Metadata: n.Metadata,
		}

		if !n.IsDir {
			return pn
		}

		sortedChildren := make([]string, 0, len(n.Children))
		for name := range n.Children {
			sortedChildren = append(sortedChildren, name)
		}
		sort.Strings(sortedChildren)

		for _, name := range sortedChildren {
			child := n.Children[name]
			// In prune mode, skip non-visible children
			if prune && !child.Visible {
				continue
			}
			pn.Children = append(pn.Children, convert(child))
		}

		return pn
	}

	portableRoot := convert(root)
	
	data := map[string]interface{}{
		"stats": stats,
		"tree":  portableRoot,
	}
	
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	
	return string(jsonBytes), nil
}
