/*
 * Component: Traceability Header Parser
 * Block-UUID: 5b4112f0-926d-4baf-870f-b056f5c667fd
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: A utility to parse the metadata header from traceable code blocks. It handles various comment styles and extracts fields into a CodeMetadata struct.
 * Language: Go
 * Created-at: 2026-02-26T01:18:45.667Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package traceability

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/gitsense/gsc-cli/internal/manifest"
)

// ParseHeader extracts the metadata header and the remaining code from a string.
// It expects the header to be at the very beginning of the content.
func ParseHeader(content string) (*manifest.CodeMetadata, string, error) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	metadata := &manifest.CodeMetadata{}
	
	var headerLines []string
	var codeLines []string
	inHeader := false
	headerFound := false
	
	// Detect comment style and extract header lines
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !headerFound {
			// Detect start of header
			if strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, "=begin") {
				inHeader = true
				continue
			}
			// Handle single-line comment styles (Bash/SQL)
			if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "--") {
				headerLines = append(headerLines, trimmed)
				continue
			}
		}

		if inHeader {
			// Detect end of header
			if strings.HasSuffix(trimmed, "*/") || strings.HasSuffix(trimmed, "-->") || strings.HasSuffix(trimmed, `"""`) || strings.HasSuffix(trimmed, "=end") {
				inHeader = false
				headerFound = true
				continue
			}
			headerLines = append(headerLines, trimmed)
			continue
		}

		// If we are past the header, collect the code
		if headerFound || (!inHeader && len(headerLines) > 0) {
			codeLines = append(codeLines, line)
		}
	}

	if len(headerLines) == 0 {
		return nil, content, fmt.Errorf("no traceability header found")
	}

	// Parse the extracted header lines
	for _, line := range headerLines {
		// Strip common comment prefixes
		cleanLine := strings.TrimPrefix(line, "* ")
		cleanLine = strings.TrimPrefix(cleanLine, "# ")
		cleanLine = strings.TrimPrefix(cleanLine, "-- ")
		cleanLine = strings.TrimSpace(cleanLine)

		if cleanLine == "" {
			continue
		}

		parts := strings.SplitN(cleanLine, ": ", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "component":
			metadata.Component = val
		case "block-uuid":
			metadata.BlockUUID = val
		case "parent-uuid":
			metadata.ParentUUID = val
		case "version":
			metadata.Version = val
		case "description":
			metadata.Description = val
		case "language":
			metadata.Language = val
		case "created-at":
			t, err := time.Parse(time.RFC3339, val)
			if err == nil {
				metadata.CreatedAt = t
			}
		case "authors":
			metadata.Authors = val
		}
	}

	// Join code lines, ensuring we respect the "two blank lines" rule if present
	code := strings.Join(codeLines, "\n")
	code = strings.TrimPrefix(code, "\n\n") // Remove the mandatory separation lines

	return metadata, code, nil
}

// ExtractUUIDs is a convenience function to get just the Block and Parent UUIDs.
func ExtractUUIDs(content string) (blockUUID, parentUUID string, err error) {
	meta, _, err := ParseHeader(content)
	if err != nil {
		return "", "", err
	}
	return meta.BlockUUID, meta.ParentUUID, nil
}
