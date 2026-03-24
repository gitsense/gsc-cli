/**
 * Component: Context Parser
 * Block-UUID: ff333a95-13f0-42ee-828f-a4e71f3cf5c0
 * Parent-UUID: 4397dd26-a9ab-4809-a376-4df405df1e9d
 * Version: 1.1.0
 * Description: Provides utilities for extracting, deduplicating, and formatting context files from chat messages. Implements deterministic sorting by Chat ID and markdown escaping for cache-optimized bucket construction.
 * Language: Go
 * Created-at: 2026-03-24T05:20:01.607Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


package context

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

// ExtractContextFiles processes all messages in the provided slice, extracting
// context files and deduplicating them by Chat ID. The latest occurrence of a
// Chat ID in the message sequence wins. The result is sorted by Chat ID.
func ExtractContextFiles(messages []db.Message) []ContextFile {
	fileMap := make(map[int64]ContextFile)

	for _, msg := range messages {
		if msg.Type == "context" && msg.Message.Valid {
			files := ParseContextMessage(msg.Message.String)
			for _, file := range files {
				// Deduplicate by Chat ID - last occurrence wins
				fileMap[file.ChatID] = file
			}
		}
	}

	// Convert map to slice
	result := make([]ContextFile, 0, len(fileMap))
	for _, file := range fileMap {
		result = append(result, file)
	}

	// CRITICAL: Sort by Chat ID for deterministic output to ensure cache stability
	sort.Slice(result, func(i, j int) bool {
		return result[i].ChatID < result[j].ChatID
	})

	return result
}

// ParseContextMessage parses a single context message string and extracts all file entries.
func ParseContextMessage(content string) []ContextFile {
	// Split by markers to isolate the items section
	parts := strings.Split(content, "\n---Start of Context---\n\n")
	if len(parts) < 2 {
		return []ContextFile{}
	}

	itemsSection := parts[1]
	sections := strings.Split(itemsSection, "\n---End of Item---\n")

	var files []ContextFile
	for _, section := range sections {
		if strings.TrimSpace(section) == "" {
			continue
		}

		file, err := ParseContextSection(section)
		if err == nil {
			files = append(files, file)
		}
	}

	return files
}

// ParseContextSection parses an individual file section within a context message.
func ParseContextSection(section string) (ContextFile, error) {
	var file ContextFile

	// Extract filename from #### `filename` line
	nameMatch := regexp.MustCompile(`(?m)^####\s*\x60([^\x60]+)\x60`).FindStringSubmatch(section)
	if len(nameMatch) < 2 {
		return file, fmt.Errorf("could not parse filename from section")
	}
	file.Name = nameMatch[1]

	// Extract metadata lines (lines starting with '-')
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}

		// Parse key: value
		parts := strings.SplitN(trimmed[1:], ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch strings.ToLower(key) {
		case "repo":
			file.Repo = value
		case "path":
			file.Path = value
		case "chat id":
			if chatID, err := strconv.ParseInt(value, 10, 64); err == nil {
				file.ChatID = chatID
			}
		case "size":
			// Handle human readable sizes if necessary, but usually raw bytes in metadata
			// Stripping " bytes", " KB" etc if present
			val := strings.Fields(value)[0]
			if size, err := strconv.Atoi(val); err == nil {
				file.Size = size
			}
		case "tokens":
			val := strings.ReplaceAll(value, ",", "")
			if tokens, err := strconv.Atoi(val); err == nil {
				file.Tokens = tokens
			}
		}
	}

	// Extract code block using the internal markdown parser
	result, err := markdown.ExtractCodeBlocks(section, false)
	if err != nil {
		return file, fmt.Errorf("failed to extract code blocks: %w", err)
	}

	if len(result.Blocks) == 0 {
		return file, fmt.Errorf("no code blocks found for %s", file.Name)
	}

	// Use the first code block found in the section
	block := result.Blocks[0]
	file.Content = block.Reconstruct()
	file.Language = block.Language

	return file, nil
}

// EscapeCodeBlocks escapes backticks in code blocks to prevent premature termination
// of markdown fences. Returns the escaped content and the 1-based line numbers of escaped lines.
func EscapeCodeBlocks(content string) (string, []int) {
	var escapedLines []int
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			lines[i] = "\\" + line
			escapedLines = append(escapedLines, i+1)
		}
	}
	return strings.Join(lines, "\n"), escapedLines
}

// FormatFileForBucket formats a ContextFile into the standardized markdown format
// used within context buckets, including metadata and escaped code blocks.
func FormatFileForBucket(file ContextFile) string {
	escapedContent, escapedLines := EscapeCodeBlocks(file.Content)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#### `%s`\n", file.Name))
	sb.WriteString(fmt.Sprintf("- Repo: %s\n", file.Repo))
	sb.WriteString(fmt.Sprintf("- Path: %s\n", file.Path))
	sb.WriteString(fmt.Sprintf("- Size: %d\n", file.Size))
	sb.WriteString(fmt.Sprintf("- Chat ID: %d\n", file.ChatID))

	if len(escapedLines) > 0 {
		var lineStrs []string
		for _, l := range escapedLines {
			lineStrs = append(lineStrs, strconv.Itoa(l))
		}
		sb.WriteString(fmt.Sprintf("- Escaped Lines: %s\n", strings.Join(lineStrs, ",")))
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("```%s\n", file.Language))
	sb.WriteString(escapedContent)
	if !strings.HasSuffix(escapedContent, "\n") {
		sb.WriteString("\n")
	}
	sb.WriteString("```\n")

	return sb.String()
}

// GenerateBucketHeader generates the standardized metadata header for a context bucket.
func GenerateBucketHeader(files []ContextFile) string {
	var sb strings.Builder

	totalSize := 0
	totalTokens := 0
	for _, file := range files {
		totalSize += file.Size
		totalTokens += file.Tokens
	}

	sb.WriteString("## FILE CONTENT - WORKING DIRECTORY\n\n")
	sb.WriteString("The following files are provided as context for your request. ")
	sb.WriteString("Use this information to understand the project's structure and content. ")
	sb.WriteString("This is a data payload for context; please do not mirror this format in your response.\n\n")

	sb.WriteString(fmt.Sprintf("**Summary:** %d file%s (%s, %s tokens)\n\n",
		len(files),
		plural(len(files)),
		formatBytes(totalSize),
		formatNumber(totalTokens)))

	// List first 10 files in the summary
	maxSummary := 10
	for i, file := range files {
		if i >= maxSummary {
			sb.WriteString(fmt.Sprintf("- ... and %d more\n", len(files)-maxSummary))
			break
		}
		sb.WriteString(fmt.Sprintf("- %s - %s, %s tokens\n",
			file.Name,
			formatBytes(file.Size),
			formatNumber(file.Tokens)))
	}

	sb.WriteString("\n---Start of Context---\n\n")

	return sb.String()
}

// Helper functions

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func formatNumber(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ",")
}

func formatBytes(bytes int) string {
	if bytes == 0 {
		return "0 bytes"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d bytes", bytes)
	}
	div, exp := int64(bytes), 0
	for n := div / unit; n >= unit; n /= unit {
		div /= unit
		exp++
	}
	units := []string{"bytes", "KB", "MB", "GB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	value := float64(bytes) / float64(int64(1)<<(uint(exp)*10))
	return fmt.Sprintf("%.1f %s", value, units[exp])
}
