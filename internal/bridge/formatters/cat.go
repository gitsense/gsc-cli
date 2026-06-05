/**
 * Component: Cat Formatter
 * Block-UUID: 9b34a743-e870-4d4a-abe5-b9d2b5b615d7
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Built-in formatter for the 'cat' command. Detects file language, extracts traceability metadata, escapes backticks, and wraps output in a Markdown code block.
 * Language: Go
 * Created-at: 2026-02-28T16:47:12.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package formatters

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gitsense/gsc-cli/internal/traceability"
)

// CatFormatter implements the Formatter interface for the 'cat' command.
type CatFormatter struct {
	filePath string
}

// PreProcess captures the file path from the arguments for later use in PostProcess.
func (cf *CatFormatter) PreProcess(args []string) []string {
	if len(args) > 0 {
		cf.filePath = args[0]
	}
	return args
}

// PostProcess enriches the raw file content with Markdown formatting and traceability info.
func (cf *CatFormatter) PostProcess(rawOutput string) (string, error) {
	// 1. Detect Language
	lang := detectLanguage(cf.filePath)

	// 2. Parse Traceability Header
	// We try to parse the header. If successful, we get the metadata and the code body.
	// If not, we treat the entire output as code.
	metadata, codeBody, err := traceability.ParseHeader(rawOutput)
	if err != nil {
		// No valid header found, use raw output as code body
		codeBody = rawOutput
		metadata = nil
	}

	// 3. Escape Backticks
	// We replace triple backticks with escaped versions to prevent breaking the chat UI.
	// Using HTML entities is safer than backslash escaping in some Markdown renderers,
	// but backslash escaping is standard for code blocks.
	escapedBody := strings.ReplaceAll(codeBody, "```", "\\```")

	// 4. Construct Markdown
	var sb strings.Builder

	// Header Line
	sb.WriteString(fmt.Sprintf("**File:** `%s`", cf.filePath))

	// Traceability Info
	if metadata != nil && metadata.BlockUUID != "" {
		sb.WriteString(fmt.Sprintf(" (Traceable: Yes | UUID: %s)", metadata.BlockUUID))
	} else {
		sb.WriteString(" (Traceable: No)")
	}
	sb.WriteString("\n\n")

	// Code Block
	sb.WriteString(fmt.Sprintf("```%s\n", lang))
	sb.WriteString(escapedBody)
	sb.WriteString("```\n")

	return sb.String(), nil
}

// detectLanguage maps a file extension to a Markdown language identifier.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	
	// Common mappings
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".php":
		return "php"
	case ".rb":
		return "ruby"
	case ".sh", ".bash":
		return "bash"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".json":
		return "json"
	case ".xml":
		return "xml"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".txt":
		return "text"
	default:
		return "text"
	}
}

// init registers the CatFormatter for the "cat" command.
func init() {
	RegisterFormatter("cat", &CatFormatter{})
}
