/*
 * Component: Tree Dump Writer
 * Block-UUID: da8312de-3dd5-4543-94b5-bb99f1d7fe82
 * Parent-UUID: ee69a04d-81f9-4f2c-9179-7ea0a0e8cce8
 * Version: 1.8.0
 * Description: Implemented WriteProvenance as a no-op to maintain compliance with the updated DumpWriter interface. The 'tree' type does not utilize provenance files.
 * Language: Go
 * Created-at: 2026-03-03T07:38:31.693Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), Gemini 3 Flash (v1.6.0), Gemini 3 Flash (v1.7.0), Gemini 3 Flash (v1.8.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/markdown"
)

const maxChatNameLength = 20

// TreeWriter implements the DumpWriter interface for the conversational tree strategy.
type TreeWriter struct{}

// Prepare wipes the output directory to ensure a clean state for the new dump.
func (w *TreeWriter) Prepare(outputDir string) error {
	// Remove existing directory if it exists
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to clean output directory: %w", err)
	}
	// Create fresh directory
	return os.MkdirAll(outputDir, 0755)
}

// GetMessageDir generates the directory path for a specific message.
// Format: chat_<id>_<truncated_name>/<visible_index>_<abbreviated_role>/
func (w *TreeWriter) GetMessageDir(chat db.Chat, msg db.Message, visibleIndex int) string {
	safeName := formatChatName(chat.Name)
	role := abbreviateRole(msg.Role)
	return fmt.Sprintf("chat_%d_%s/%03d_%s", chat.ID, safeName, visibleIndex, role)
}

// WriteMessage persists the raw markdown content of the message.
func (w *TreeWriter) WriteMessage(msgDir string, msg db.Message) error {
	path := filepath.Join(msgDir, "message.md")
	return os.WriteFile(path, []byte(msg.Message.String), 0644)
}

// WriteProvenance is a no-op for the TreeWriter as provenance files are specific to the MergedWriter.
func (w *TreeWriter) WriteProvenance(msgDir string, chats []db.Chat) error {
	return nil
}

// WriteBlock persists a code block, including its traceability header.
// The trim flag is passed to satisfy the interface, but the actual trimming
// is handled by the parser before this method is called.
func (w *TreeWriter) WriteBlock(msgDir string, block markdown.CodeBlock, trim bool) error {
	// Determine filename: block_<idx>_<uuid>.ext or block_<idx>.ext
	filename := fmt.Sprintf("%03d_block", block.Index + 1)
	if block.BlockUUID != "" {
		// Truncate UUID to first 8 characters for readability
		filename += "_" + block.BlockUUID[:8]
	}
	filename += getExtension(block.Language)

	// Combine header and code for full context
	// Note: block.ExecutableCode is already processed (trimmed or raw) by the parser
	content := block.RawHeader + "\n\n\n" + block.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// WritePatch persists a patch block, including its metadata header.
// The trim flag is passed to satisfy the interface, but the actual trimming
// is handled by the parser before this method is called.
func (w *TreeWriter) WritePatch(msgDir string, patch markdown.PatchBlock, trim bool) error {
	// Determine filename: patch_<idx>_<uuid>.diff or patch_<idx>.diff
	filename := fmt.Sprintf("%03d_patch", patch.Index + 1)
	if patch.TargetBlockUUID != "" {
		// Truncate UUID to first 8 characters for readability
		filename += "_" + patch.TargetBlockUUID[:8]
	}
	filename += ".diff"

	// Combine header and diff content
	// Note: patch.ExecutableCode is already processed (trimmed or raw) by the parser
	content := patch.RawHeader + "\n\n\n" + patch.ExecutableCode
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(content), 0644)
}

// WritePatchedFile persists the result of a successful patch application.
// This is a "result" file and does not include the GitSense metadata header.
func (w *TreeWriter) WritePatchedFile(msgDir string, patch markdown.PatchBlock, header string, content string) error {
	// Determine filename: patched_<idx>_<uuid>.ext
	filename := fmt.Sprintf("%03d_patched", patch.Index + 1)
	if patch.TargetBlockUUID != "" {
		// Truncate UUID to first 8 characters for readability
		filename += "_" + patch.TargetBlockUUID[:8]
	}
	
	// We use the language from the patch block to determine the extension
	filename += getExtension(patch.Language)

	fullContent := header + "\n\n\n" + content
	path := filepath.Join(msgDir, filename)
	return os.WriteFile(path, []byte(fullContent), 0644)
}

// formatChatName sanitizes and truncates the chat name to a maximum length.
func formatChatName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	safe := reg.ReplaceAllString(name, "_")
	
	if len(safe) > maxChatNameLength {
		return safe[:maxChatNameLength] + "..."
	}
	return safe
}

// abbreviateRole maps full role names to 4-character codes.
func abbreviateRole(role string) string {
	switch strings.ToLower(role) {
	case "assistant":
		return "asst"
	case "system":
		return "syst"
	case "user":
		return "user"
	default:
		// Fallback: take first 4 chars
		if len(role) > 4 {
			return role[:4]
		}
		return role
	}
}

// getExtension maps a language string to a file extension.
func getExtension(lang string) string {
	switch strings.ToLower(lang) {
	// Web & Frontend
	case "javascript", "js":
		return ".js"
	case "typescript", "ts":
		return ".ts"
	case "html":
		return ".html"
	case "css":
		return ".css"
	case "scss":
		return ".scss"
	case "sass":
		return ".sass"
	case "less":
		return ".less"
	case "jsx":
		return ".jsx"
	case "tsx":
		return ".tsx"
	case "vue":
		return ".vue"

	// Backend & Systems
	case "go":
		return ".go"
	case "python", "py":
		return ".py"
	case "rust", "rs":
		return ".rs"
	case "c":
		return ".c"
	case "cpp", "cc", "cxx", "c++":
		return ".cpp"
	case "h", "hpp":
		return ".h"
	case "java":
		return ".java"
	case "kotlin", "kt":
		return ".kt"
	case "swift":
		return ".swift"
	case "objective-c", "objc":
		return ".m"
	case "csharp", "cs":
		return ".cs"
	case "php":
		return ".php"
	case "ruby", "rb":
		return ".rb"
	case "perl", "pl":
		return ".pl"
	case "lua":
		return ".lua"
	case "elixir", "ex":
		return ".ex"
	case "erlang", "erl":
		return ".erl"
	case "clojure", "clj":
		return ".clj"
	case "scala":
		return ".scala"
	case "groovy":
		return ".groovy"
	case "haskell", "hs":
		return ".hs"
	case "ocaml", "ml":
		return ".ml"
	case "fsharp", "fs":
		return ".fs"
	case "go-template", "gotmpl":
		return ".gotmpl"

	// Data & Config
	case "json":
		return ".json"
	case "yaml", "yml":
		return ".yml"
	case "toml":
		return ".toml"
	case "xml":
		return ".xml"
	case "csv":
		return ".csv"
	case "ini", "cfg":
		return ".ini"
	case "properties":
		return ".properties"

	// Database & Query
	case "sql":
		return ".sql"
	case "plsql", "tsql":
		return ".sql"
	case "graphql", "gql":
		return ".graphql"
	case "mongodb":
		return ".js" // MongoDB uses JavaScript-like syntax

	// Markup & Documentation
	case "markdown", "md":
		return ".md"
	case "rst":
		return ".rst"
	case "asciidoc", "adoc":
		return ".adoc"
	case "tex", "latex":
		return ".tex"

	// Shell & Scripts
	case "bash", "sh", "shell":
		return ".sh"
	case "zsh":
		return ".zsh"
	case "fish":
		return ".fish"
	case "powershell", "ps1":
		return ".ps1"
	case "batch", "cmd", "dos":
		return ".bat"

	// Version Control & Build
	case "diff", "patch":
		return ".diff"
	case "dockerfile":
		return ".dockerfile"
	case "makefile":
		return ".makefile"
	case "cmake":
		return ".cmake"
	case "gradle":
		return ".gradle"
	case "maven", "pom":
		return ".xml"

	// Other
	case "regex", "regexp":
		return ".regex"
	case "vim", "viml":
		return ".vim"
	case "emacs", "elisp":
		return ".el"
	case "lisp":
		return ".lisp"
	case "scheme":
		return ".scm"
	case "prolog":
		return ".pl"
	case "r":
		return ".r"
	case "matlab":
		return ".m"
	case "octave":
		return ".m"
	case "julia":
		return ".jl"
	case "dart":
		return ".dart"
	case "go-mod":
		return ".mod"
	case "go-sum":
		return ".sum"

	default:
		return ".txt"
	}
}
