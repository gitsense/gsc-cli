/*
 * Component: Language Detector
 * Block-UUID: e8345737-35ef-4b90-a300-024744c38075
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Provides file language detection by file extension and well-known filename without external dependencies. Returns a human-readable language name for use in change metadata enrichment.
 * Language: Go
 * Created-at: 2026-04-25T03:24:12.891Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0)
 */


package intent_workflow

import (
	"path/filepath"
	"strings"
)

var extToLanguage = map[string]string{
	".go":      "Go",
	".py":      "Python",
	".js":      "JavaScript",
	".jsx":     "JavaScript",
	".ts":      "TypeScript",
	".tsx":     "TypeScript",
	".rb":      "Ruby",
	".java":    "Java",
	".c":       "C",
	".cc":      "C++",
	".cpp":     "C++",
	".cxx":     "C++",
	".h":       "C",
	".hpp":     "C++",
	".cs":      "C#",
	".php":     "PHP",
	".swift":   "Swift",
	".kt":      "Kotlin",
	".rs":      "Rust",
	".scala":   "Scala",
	".md":      "Markdown",
	".json":    "JSON",
	".yaml":    "YAML",
	".yml":     "YAML",
	".toml":    "TOML",
	".sh":      "Bash",
	".bash":    "Bash",
	".zsh":     "Bash",
	".fish":    "Fish",
	".sql":     "SQL",
	".html":    "HTML",
	".htm":     "HTML",
	".css":     "CSS",
	".scss":    "SCSS",
	".less":    "LESS",
	".xml":     "XML",
	".tf":      "Terraform",
	".proto":   "Protocol Buffers",
	".graphql": "GraphQL",
	".gql":     "GraphQL",
	".lua":     "Lua",
	".r":       "R",
	".ex":      "Elixir",
	".exs":     "Elixir",
	".erl":     "Erlang",
	".hs":      "Haskell",
	".clj":     "Clojure",
}

var filenameToLanguage = map[string]string{
	"Makefile":    "Makefile",
	"Dockerfile":  "Dockerfile",
	"Vagrantfile": "Ruby",
	"Jenkinsfile": "Groovy",
	"Gemfile":     "Ruby",
	"Rakefile":    "Ruby",
	".gitignore":  "Git Config",
	".env":        "Shell",
}

// DetectLanguage returns a human-readable language name for the given file path.
// Returns "Unknown" if the language cannot be determined from the extension or filename.
func DetectLanguage(filePath string) string {
	base := filepath.Base(filePath)

	if lang, ok := filenameToLanguage[base]; ok {
		return lang
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "Unknown"
	}

	if lang, ok := extToLanguage[ext]; ok {
		return lang
	}

	return "Unknown"
}
