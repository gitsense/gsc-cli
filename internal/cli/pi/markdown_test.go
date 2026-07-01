/**
 * Component: Preview Markdown Renderer Tests
 * Block-UUID: d4e5f6a7-b8c9-0123-defg-456789012345
 * Parent-UUID: c3d4e5f6-a7b8-9012-cdef-345678901234
 * Version: 1.0.0
 * Description: Unit tests for the markdown renderer used in the resume picker preview pane.
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package pi

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Empty(t *testing.T) {
	result := renderMarkdown("", 60)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestRenderMarkdown_SingleParagraph(t *testing.T) {
	input := "Hello world"
	result := renderMarkdown(input, 60)
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	if !strings.Contains(result[0], "Hello world") {
		t.Errorf("expected 'Hello world' in output, got %q", result[0])
	}
}

func TestRenderMarkdown_MultipleParagraphs(t *testing.T) {
	input := "First paragraph.\n\nSecond paragraph."
	result := renderMarkdown(input, 60)
	// Should have: line1, empty, line2
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(result), result)
	}
	if result[1] != "" {
		t.Errorf("expected empty line between paragraphs, got %q", result[1])
	}
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	input := "Example:\n\n```bash\ngsc pi guide\n```"
	result := renderMarkdown(input, 60)
	// Should have: "Example:", empty, code line
	if len(result) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(result), result)
	}
	if !strings.Contains(result[2], "gsc pi guide") {
		t.Errorf("expected code in output, got %q", result[2])
	}
}

func TestRenderMarkdown_List(t *testing.T) {
	input := "Items:\n\n- First item\n- Second item"
	result := renderMarkdown(input, 60)
	// Should have: "Items:", empty, bullet1, bullet2
	if len(result) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(result), result)
	}
	if !strings.Contains(result[2], "•") {
		t.Errorf("expected bullet point, got %q", result[2])
	}
}

func TestRenderMarkdown_BoldText(t *testing.T) {
	input := "This is **bold** text"
	result := renderMarkdown(input, 60)
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	// The rendered output should contain the bold text
	if !strings.Contains(result[0], "bold") {
		t.Errorf("expected 'bold' in output, got %q", result[0])
	}
}

func TestRenderMarkdown_InlineCode(t *testing.T) {
	input := "Use `gsc pi guide` command"
	result := renderMarkdown(input, 60)
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	// The rendered output should contain the code
	if !strings.Contains(result[0], "gsc pi guide") {
		t.Errorf("expected 'gsc pi guide' in output, got %q", result[0])
	}
}

func TestRenderMarkdown_MixedContent(t *testing.T) {
	input := `Done. Here's what I created:

**New files:**
- ` + "`guide.go`" + ` — Command implementation

**Usage:**
` + "```bash" + `
gsc pi guide
` + "```"

	result := renderMarkdown(input, 60)
	if len(result) < 5 {
		t.Fatalf("expected at least 5 lines, got %d: %v", len(result), result)
	}
	
	// Check that paragraphs are separated
	foundEmpty := false
	for _, line := range result {
		if line == "" {
			foundEmpty = true
			break
		}
	}
	if !foundEmpty {
		t.Error("expected empty line between paragraphs")
	}
}
