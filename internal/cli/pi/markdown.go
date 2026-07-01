/**
 * Component: Preview Markdown Renderer
 * Block-UUID: c3d4e5f6-a7b8-9012-cdef-345678901234
 * Parent-UUID: c4d9e1f6-8a32-4b07-9d15-6f2e3a8c0b91
 * Version: 1.0.0
 * Description: Lightweight markdown renderer for the resume picker preview pane. Renders code blocks, bold text, inline code, and lists with terminal styling using lipgloss.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package pi

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Markdown rendering styles
var (
	styleCodeBlock = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	styleInlineCode = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252"))

	styleBold = lipgloss.NewStyle().Bold(true)

	styleListBullet = lipgloss.NewStyle().
			Foreground(lipgloss.Color("208"))

	styleHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))
)

// Regex patterns for markdown elements
var (
	reCodeBlock   = regexp.MustCompile("(?s)```(?:\\w*)\\n(.*?)\\n```")
	reInlineCode  = regexp.MustCompile("`([^`]+)`")
	reBold        = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reHeader      = regexp.MustCompile(`^(#{1,3})\s+(.+)$`)
	reListItem    = regexp.MustCompile(`^(\s*)[-*]\s+(.+)$`)
)

// renderMarkdown converts markdown text to styled terminal output.
// It preserves paragraph structure and renders common markdown elements.
func renderMarkdown(text string, width int) []string {
	if text == "" {
		return nil
	}

	// Split into paragraphs (double newline)
	paragraphs := strings.Split(text, "\n\n")
	var result []string

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Add spacing between paragraphs
		if i > 0 {
			result = append(result, "")
		}

		lines := renderParagraph(para, width)
		result = append(result, lines...)
	}

	return result
}

// renderParagraph handles a single paragraph, detecting and rendering
// code blocks, lists, and inline formatting.
func renderParagraph(text string, width int) []string {
	// Check if this is a code block
	if strings.HasPrefix(text, "```") {
		return renderCodeBlock(text, width)
	}

	// Check if this is a list
	if isList(text) {
		return renderList(text, width)
	}

	// Regular paragraph with inline formatting
	return renderInlineFormatted(text, width)
}

// renderCodeBlock renders a fenced code block with background styling.
func renderCodeBlock(text string, width int) []string {
	// Extract language and code
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return renderInlineFormatted(text, width)
	}

	// Remove opening and closing fences
	codeLines := lines[1:]
	if len(codeLines) > 0 && strings.HasPrefix(codeLines[len(codeLines)-1], "```") {
		codeLines = codeLines[:len(codeLines)-1]
	}

	// Render each code line with background
	var result []string
	for _, line := range codeLines {
		// Truncate if too long
		if len(line) > width-4 {
			line = line[:width-7] + "..."
		}
		rendered := styleCodeBlock.Render(" " + line)
		result = append(result, rendered)
	}

	return result
}

// isList checks if text is a markdown list.
func isList(text string) bool {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return false
	}
	// Check if first line starts with a list marker
	return reListItem.MatchString(lines[0])
}

// renderList renders a markdown list with bullet styling.
func renderList(text string, width int) []string {
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for list item
		matches := reListItem.FindStringSubmatch(line)
		if len(matches) >= 3 {
			bullet := styleListBullet.Render("• ")
			content := renderInline(matches[2])
			// Wrap long list items
			wrapped := wrap(bullet+content, width)
			for j, wl := range wrapped {
				if j == 0 {
					result = append(result, wl)
				} else {
					result = append(result, "  "+wl)
				}
			}
		} else {
			// Not a list item, render as regular text
			rendered := renderInline(line)
			wrapped := wrap(rendered, width)
			for _, wl := range wrapped {
				result = append(result, wl)
			}
		}
	}

	return result
}

// renderInlineFormatted renders a paragraph with inline markdown formatting.
func renderInlineFormatted(text string, width int) []string {
	// Split by newlines to preserve line structure
	lines := strings.Split(text, "\n")
	var result []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for header
		if headerMatch := reHeader.FindStringSubmatch(line); len(headerMatch) >= 3 {
			rendered := styleHeader.Render(headerMatch[2])
			result = append(result, rendered)
			continue
		}

		// Render inline formatting
		rendered := renderInline(line)
		wrapped := wrap(rendered, width)
		for _, wl := range wrapped {
			result = append(result, wl)
		}
	}

	return result
}

// renderInline applies inline markdown formatting (bold, code).
func renderInline(text string) string {
	// Render inline code
	text = reInlineCode.ReplaceAllStringFunc(text, func(match string) string {
		code := strings.Trim(match, "`")
		return styleInlineCode.Render(code)
	})

	// Render bold text
	text = reBold.ReplaceAllStringFunc(text, func(match string) string {
		bold := strings.Trim(match, "*")
		return styleBold.Render(bold)
	})

	return text
}
