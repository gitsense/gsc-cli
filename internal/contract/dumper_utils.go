/**
 * Component: Dumper Utilities
 * Block-UUID: e59122df-ce42-4e5f-a659-d7bb135c468a
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Utility functions for the contract dumper, including hashing, sanitization, and header generation.
 * Language: Go
 * Created-at: 2026-03-10T00:20:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// calculateMessageHash generates a deterministic hash for a message.
func calculateMessageHash(msg db.Message) string {
	h := sha256.New()
	h.Write([]byte(msg.Role))
	if msg.Message.Valid {
		h.Write([]byte(msg.Message.String))
	}
	h.Write([]byte(msg.CreatedAt.Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// sanitizeComponentName sanitizes a component name for use in file paths.
// It replaces spaces with underscores, removes special characters, and lowercases the result.
func sanitizeComponentName(name string) string {
	// Replace spaces with underscores
	sanitized := strings.ReplaceAll(name, " ", "_")
	
	// Remove special characters (keep only: a-z, A-Z, 0-9, -, _)
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	sanitized = reg.ReplaceAllString(sanitized, "")
	
	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)
	
	// Ensure non-empty result
	if sanitized == "" {
		sanitized = "component"
	}
	
	return sanitized
}

// generateCodeHeaderFromPatch constructs a Code Block header from a PatchBlock.
// This is used to create the header for .patched files, which represent a new version of the code.
func generateCodeHeaderFromPatch(patch markdown.PatchBlock) string {
	lang := strings.ToLower(patch.Language)

	// Define comment styles based on GitSense specifications
	var start, end, linePrefix string

	switch {
	case lang == "python":
		start = `"""`
		end = `"""`
		linePrefix = ""
	case lang == "ruby":
		start = "=begin"
		end = "=end"
		linePrefix = ""
	case lang == "bash" || lang == "sh" || lang == "zsh" || lang == "fish":
		start = ""
		end = ""
		linePrefix = "# "
	case lang == "html" || lang == "xml" || lang == "svg" || lang == "markdown" || lang == "md":
		start = "<!--"
		end = "-->"
		linePrefix = ""
	case lang == "sql":
		start = ""
		end = ""
		linePrefix = "-- "
	default:
		// Default to C-style (C, C++, Go, Java, JS, TS, etc.)
		start = "/*"
		end = "*/"
		linePrefix = " * "
	}

	var sb strings.Builder

	// Start block
	if start != "" {
		sb.WriteString(start + "\n")
	}

	// Write fields
	writeField := func(key, value string) {
		if linePrefix != "" {
			sb.WriteString(fmt.Sprintf("%s%s: %s\n", linePrefix, key, value))
		} else {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	writeField("Component", patch.Component)
	writeField("Block-UUID", patch.TargetBlockUUID)
	writeField("Parent-UUID", patch.SourceBlockUUID)
	writeField("Version", patch.TargetVersion)
	writeField("Description", patch.Description)
	writeField("Language", patch.Language)
	writeField("Created-at", patch.CreatedAt)
	writeField("Authors", strings.Join(patch.Authors, ", "))

	// End block
	if end != "" {
		// Align the closing delimiter for C-style comments
		if start == "/*" {
			sb.WriteString(" " + end)
		} else {
			sb.WriteString(end)
		}
	}

	return sb.String()
}
