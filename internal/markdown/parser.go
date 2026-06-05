/**
 * Component: Markdown Parser Utility
 * Block-UUID: 7b677193-8b29-43a0-acaf-c1afc6eb714d
 * Parent-UUID: e4b4e576-10da-42a2-b788-d4f55be5a35b
 * Version: 1.7.0
 * Description: Added smart trimming logic to splitHeaderAndCode. It now removes leading/trailing blank lines and trailing whitespace from executable code while preserving semantic indentation. The ExtractCodeBlocks function now accepts a trim boolean to control this behavior.
 * Language: Go
 * Created-at: 2026-03-24T05:18:33.296Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), Gemini 3 Flash (v1.3.0), Gemini 3 Flash (v1.4.0), Gemini 3 Flash (v1.5.0), Gemini 3 Flash (v1.5.1), GLM-4.7 (v1.5.2), Gemini 3 Flash (v1.6.0), GLM-4.7 (v1.7.0)
 */


package markdown

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// CodeBlock represents a full code snapshot.
type CodeBlock struct {
	Index          int      `json:"index"`
	Language       string   `json:"language"`
	RawHeader      string   `json:"raw_header"`      // The exact text of the metadata header (comments only)
	ExecutableCode string   `json:"executable_code"` // The code after the two blank lines
	BlockUUID      string   `json:"block_uuid,omitempty"`
	ParentUUID     string   `json:"parent_uuid,omitempty"`
	Version        string   `json:"version,omitempty"`
	Component      string   `json:"component,omitempty"`
	Description    string   `json:"description,omitempty"`
	Authors        []string `json:"authors,omitempty"`
	CreatedAt      string   `json:"created_at,omitempty"`
}

// PatchBlock represents an incremental change (diff).
type PatchBlock struct {
	Index           int      `json:"index"`
	Language        string   `json:"language"` // Usually "diff"
	RawHeader       string   `json:"raw_header"`
	ExecutableCode  string   `json:"executable_code"` // The diff content after the two blank lines
	SourceBlockUUID string   `json:"source_block_uuid,omitempty"`
	TargetBlockUUID string   `json:"target_block_uuid,omitempty"`
	SourceVersion   string   `json:"source_version,omitempty"`
	TargetVersion   string   `json:"target_version,omitempty"`
	Component       string   `json:"component,omitempty"`
	Description     string   `json:"description,omitempty"`
	Authors         []string `json:"authors,omitempty"`
	CreatedAt       string   `json:"created_at,omitempty"`
}

// ParseResult contains all artifacts extracted from a markdown message.
type ParseResult struct {
	Blocks  []CodeBlock  `json:"blocks"`
	Patches []PatchBlock `json:"patches"`
}

// Regex patterns for metadata extraction
var (
	reBlockUUID   = regexp.MustCompile(`(?m)Block-UUID:\s*([a-fA-F0-9-]{36}|{{GS-UUID}})`)
	reParentUUID  = regexp.MustCompile(`(?m)Parent-UUID:\s*([a-fA-F0-9-]{36}|N/A)`)
	reVersion     = regexp.MustCompile(`(?m)Version:\s*(\d+\.\d+\.\d+)`)
	reComponent   = regexp.MustCompile(`(?m)Component:\s*(.*)`)
	reDescription = regexp.MustCompile(`(?m)Description:\s*(.*)`)
	reAuthors     = regexp.MustCompile(`(?m)Authors:\s*(.*)`)
	reCreatedAt   = regexp.MustCompile(`(?m)Created-at:\s*(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?)`)

	reSourceBlockUUID = regexp.MustCompile(`(?m)Source-Block-UUID:\s*([a-fA-F0-9-]{36})`)
	reTargetBlockUUID = regexp.MustCompile(`(?m)Target-Block-UUID:\s*([a-fA-F0-9-]{36}|{{GS-UUID}})`)
	reSourceVersion   = regexp.MustCompile(`(?m)Source-Version:\s*(\d+\.\d+\.\d+)`)
	reTargetVersion   = regexp.MustCompile(`(?m)Target-Version:\s*(\d+\.\d+\.\d+)`)
	rePatchLanguage   = regexp.MustCompile(`(?m)# Language:\s*(.*)`)
)

// ExtractCodeBlocks parses a markdown string and returns structured blocks and patches.
// The trim flag controls whether smart trimming is applied to the executable code.
func ExtractCodeBlocks(content string, trim bool) (*ParseResult, error) {
	md := goldmark.New()
	source := []byte(content)
	reader := text.NewReader(source)
	doc := md.Parser().Parse(reader)

	result := &ParseResult{
		Blocks:  []CodeBlock{},
		Patches: []PatchBlock{},
	}

	blockIdx := 0
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == ast.KindFencedCodeBlock {
			cb := node.(*ast.FencedCodeBlock)
			lang := string(cb.Language(source))
			
			var sb strings.Builder
			lines := cb.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				sb.Write(line.Value(source))
			}
			rawContent := sb.String()

			if lang == "diff" && strings.Contains(rawContent, "# Patch Metadata") {
				patch := parsePatchBlock(rawContent, blockIdx, trim)
				result.Patches = append(result.Patches, patch)
			} else {
				block := parseCodeBlock(rawContent, lang, blockIdx, trim)
				result.Blocks = append(result.Blocks, block)
			}

			blockIdx++
		}
		return ast.WalkContinue, nil
	})

	return result, nil
}

func parseCodeBlock(raw string, lang string, idx int, trim bool) CodeBlock {
	header, code := splitHeaderAndCode(raw, trim)
	
	block := CodeBlock{
		Index:          idx,
		Language:       lang,
		RawHeader:      header,
		ExecutableCode: code,
	}

	if m := reBlockUUID.FindStringSubmatch(header); len(m) > 1 { block.BlockUUID = m[1] }
	if m := reParentUUID.FindStringSubmatch(header); len(m) > 1 { block.ParentUUID = m[1] }
	if m := reVersion.FindStringSubmatch(header); len(m) > 1 { block.Version = m[1] }
	if m := reComponent.FindStringSubmatch(header); len(m) > 1 { block.Component = strings.TrimSpace(m[1]) }
	if m := reDescription.FindStringSubmatch(header); len(m) > 1 { block.Description = strings.TrimSpace(m[1]) }
	if m := reCreatedAt.FindStringSubmatch(header); len(m) > 1 { block.CreatedAt = m[1] }
	if m := reAuthors.FindStringSubmatch(header); len(m) > 1 {
		for _, a := range strings.Split(m[1], ",") {
			block.Authors = append(block.Authors, strings.TrimSpace(a))
		}
	}
	return block
}

func parsePatchBlock(raw string, idx int, trim bool) PatchBlock {
	header, code := splitHeaderAndCode(raw, trim)

	patch := PatchBlock{
		Index:          idx,
		Language:       "",
		RawHeader:      header,
		ExecutableCode: code,
	}

	if m := reSourceBlockUUID.FindStringSubmatch(header); len(m) > 1 { patch.SourceBlockUUID = m[1] }
	if m := reTargetBlockUUID.FindStringSubmatch(header); len(m) > 1 { patch.TargetBlockUUID = m[1] }
	if m := reSourceVersion.FindStringSubmatch(header); len(m) > 1 { patch.SourceVersion = m[1] }
	if m := reTargetVersion.FindStringSubmatch(header); len(m) > 1 { patch.TargetVersion = m[1] }
	if m := reComponent.FindStringSubmatch(header); len(m) > 1 { patch.Component = strings.TrimSpace(m[1]) }
	if m := reDescription.FindStringSubmatch(header); len(m) > 1 { patch.Description = strings.TrimSpace(m[1]) }
	if m := reCreatedAt.FindStringSubmatch(header); len(m) > 1 { patch.CreatedAt = m[1] }
	if m := reAuthors.FindStringSubmatch(header); len(m) > 1 {
		for _, a := range strings.Split(m[1], ",") {
			patch.Authors = append(patch.Authors, strings.TrimSpace(a))
		}
	}

	// Extract Language from Patch Metadata Header
	if m := rePatchLanguage.FindStringSubmatch(header); len(m) > 1 {
		patch.Language = strings.TrimSpace(m[1])
	} else {
		patch.Language = "diff" // Fallback for malformed patches
	}
	return patch
}

// splitHeaderAndCode separates the comment-based metadata header from the executable code.
// It applies "Smart Trim" to the executable code if the trim flag is true.
func splitHeaderAndCode(raw string, trim bool) (string, string) {
	// The GitSense spec mandates exactly TWO BLANK LINES between the header and code.
	// This translates to three consecutive newline characters.
	parts := strings.SplitN(raw, "\n\n\n", 2)
	
	if len(parts) < 2 {
		return "", raw
	}

	header := parts[0]
	code := parts[1]
	
	if trim {
		codeLines := strings.Split(code, "\n")
		// Trim trailing whitespace from each line
		for i := range codeLines {
			codeLines[i] = strings.TrimRight(codeLines[i], " \t\r")
		}
		// Trim trailing blank lines from the block
		lastLine := len(codeLines) - 1
		for lastLine >= 0 && strings.TrimSpace(codeLines[lastLine]) == "" {
			lastLine--
		}
		code = strings.Join(codeLines[:lastLine+1], "\n")
	}
	return header, code
}

func isCommentLine(line string) bool {
	return strings.HasPrefix(line, "/*") || 
	       strings.HasPrefix(line, "*") || 
	       strings.HasPrefix(line, "//") || 
	       strings.HasPrefix(line, "#") ||
	       strings.HasPrefix(line, "<!--") ||
	       strings.HasPrefix(line, "=") ||
	       (strings.HasPrefix(line, "--") && !strings.HasPrefix(line, "---")) // SQL but not Diff
}

// Reconstruct returns the full code block with header and executable code
// properly formatted according to GitSense standards.
func (cb *CodeBlock) Reconstruct() string {
	var sb strings.Builder
	sb.WriteString(cb.RawHeader)
	sb.WriteString("\n\n\n")  // Two blank lines as per spec
	sb.WriteString(cb.ExecutableCode)
	return sb.String()
}

// Reconstruct returns the full patch block with header and executable code
// properly formatted according to GitSense standards.
func (pb *PatchBlock) Reconstruct() string {
	var sb strings.Builder
	sb.WriteString(pb.RawHeader)
	sb.WriteString("\n\n\n")  // Two blank lines as per spec
	sb.WriteString(pb.ExecutableCode)
	return sb.String()
}
