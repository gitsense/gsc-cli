/**
 * Component: Provenance Recorder
 * Block-UUID: 5b9d50de-417b-4599-ab2c-c38f22e8d138
 * Parent-UUID: 94e22627-d8f9-43b8-9f9a-c56fa7c0b75b
 * Version: 1.3.0
 * Description: Handles the recording of code provenance to the worktree-level ledger (.gitsense/provenance.jsonl) and the injection of ephemeral code block headers into source files. Updated ReadExistingHeader to ensure all metadata fields (Block-UUID, Version, Authors, Created-at) are captured before exiting the loop, preventing premature termination when Version is found before history fields. Updated generateHeader to display "N/A" for Parent-UUID when it is empty, ensuring consistency with the planned convention.
 * Language: Go
 * Created-at: 2026-04-28T12:45:20.087Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0)
 */


package provenance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProvenanceEntry represents a single line in the .gitsense/provenance.jsonl file.
type ProvenanceEntry struct {
	Timestamp      time.Time `json:"timestamp"`
	ContractUUID   string    `json:"contract_uuid,omitempty"`
	SessionID      string    `json:"session_id"`
	TurnID         int       `json:"turn_id"`
	BlockUUID      string    `json:"block_uuid"`
	ParentUUID     string    `json:"parent_uuid,omitempty"`
	OldVersion     string    `json:"old_version,omitempty"`
	NewVersion     string    `json:"new_version"`
	Path           string    `json:"path"`            // Relative path from worktree root
	WorkingDirPath string    `json:"working_dir_path"` // Absolute path to worktree root
	OldBlobSHA     string    `json:"old_blob_sha,omitempty"`
	NewBlobSHA     string    `json:"new_blob_sha"`
	ChangeType     string    `json:"change_type"` // "added", "modified", "deleted"
	AuthorType     string    `json:"author_type"` // "ai" or "human"
	AuthorName     string    `json:"author_name"`
	ModelID        string    `json:"model_id,omitempty"`
	Source         string    `json:"source"` // "gsc-cli" | "gitsense-chat-app"
	Description    string    `json:"description"`
	LinesAdded     int       `json:"lines_added"`
	LinesDeleted   int       `json:"lines_deleted"`
}

// RecordChange appends a ProvenanceEntry to the .gitsense/provenance.jsonl file.
func RecordChange(workingDir string, entry *ProvenanceEntry) error {
	gitsenseDir := filepath.Join(workingDir, ".gitsense")
	if err := os.MkdirAll(gitsenseDir, 0755); err != nil {
		return fmt.Errorf("failed to create .gitsense directory: %w", err)
	}

	ledgerPath := filepath.Join(gitsenseDir, "provenance.jsonl")
	file, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open provenance ledger: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal provenance entry: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to provenance ledger: %w", err)
	}

	return nil
}

// InjectCodeBlockHeader prepends a language-appropriate code block header to the file.
func InjectCodeBlockHeader(filePath, language string, entry *ProvenanceEntry) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file for header injection: %w", err)
	}

	// Read existing header metadata to preserve history
	_, _, existingAuthors, existingCreatedAt, err := ReadExistingHeader(entry.OldBlobSHA, entry.WorkingDirPath)
	if err != nil {
		// If we can't read the old header (e.g. new file), proceed without history
		existingAuthors = ""
		existingCreatedAt = ""
	}

	header := generateHeader(language, entry, existingAuthors, existingCreatedAt)
	
	// Prepend header and two blank lines as per protocol
	newContent := []byte(header + "\n\n\n" + string(content))
	
	if err := os.WriteFile(filePath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write file with injected header: %w", err)
	}

	return nil
}

// ReadExistingHeader extracts the Block-UUID, Version, Authors, and Created-at from an existing header in a git blob.
func ReadExistingHeader(blobSHA, workingDir string) (blockUUID, version, authors, createdAt string, err error) {
	if blobSHA == "" {
		return "", "", "", "", nil
	}

	cmd := exec.Command("git", "cat-file", "blob", blobSHA)
	cmd.Dir = workingDir
	out, err := cmd.Output()
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to read git blob %s: %w", blobSHA, err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "Block-UUID:") {
			parts := strings.SplitN(line, "Block-UUID:", 2)
			if len(parts) == 2 {
				blockUUID = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Version:") {
			parts := strings.SplitN(line, "Version:", 2)
			if len(parts) == 2 {
				version = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Authors:") {
			parts := strings.SplitN(line, "Authors:", 2)
			if len(parts) == 2 {
				authors = strings.TrimSpace(parts[1])
			}
		}
		if strings.Contains(line, "Created-at:") {
			parts := strings.SplitN(line, "Created-at:", 2)
			if len(parts) == 2 {
				createdAt = strings.TrimSpace(parts[1])
			}
		}
		// Stop searching once we've found all four fields or passed the likely header area
		if blockUUID != "" && version != "" && authors != "" && createdAt != "" {
			break
		}
		if line != "" && !strings.HasPrefix(line, "/*") && !strings.HasPrefix(line, "*") && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "\"\"\"") && !strings.HasPrefix(line, "<!--") {
			// We've likely passed the header
			break
		}
	}

	return blockUUID, version, authors, createdAt, nil
}

// generateHeader constructs the metadata header string based on language conventions.
// It accepts existingAuthors and existingCreatedAt to preserve history across versions.
func generateHeader(language string, entry *ProvenanceEntry, existingAuthors, existingCreatedAt string) string {
	fields := []string{
		fmt.Sprintf("Component: %s", filepath.Base(entry.Path)),
		fmt.Sprintf("Block-UUID: %s", entry.BlockUUID),
		func() string {
			if entry.ParentUUID == "" {
				return "Parent-UUID: N/A"
			}
			return fmt.Sprintf("Parent-UUID: %s", entry.ParentUUID)
		}(),
		fmt.Sprintf("Version: %s", entry.NewVersion),
		fmt.Sprintf("Description: %s", entry.Description),
		fmt.Sprintf("Language: %s", language),
	}

	// Preserve original Created-at if it exists, otherwise use current timestamp
	if existingCreatedAt != "" {
		fields = append(fields, fmt.Sprintf("Created-at: %s", existingCreatedAt))
	} else {
		fields = append(fields, fmt.Sprintf("Created-at: %s", entry.Timestamp.Format(time.RFC3339Nano)))
	}

	// Append new author to existing history or start new history
	if existingAuthors != "" {
		fields = append(fields, fmt.Sprintf("Authors: %s, %s (v%s)", existingAuthors, entry.AuthorName, entry.NewVersion))
	} else {
		fields = append(fields, fmt.Sprintf("Authors: %s (v%s)", entry.AuthorName, entry.NewVersion))
	}

	switch strings.ToLower(language) {
	case "python":
		return "\"\"\"\n" + strings.Join(fields, "\n") + "\n\"\"\""
	case "bash", "shell", "yaml", "yml", "dockerfile":
		var lines []string
		for _, f := range fields {
			lines = append(lines, "# "+f)
		}
		return strings.Join(lines, "\n")
	case "markdown", "html", "xml", "svg":
		return "<!--\n" + strings.Join(fields, "\n") + "\n-->"
	case "ruby":
		return "=begin\n" + strings.Join(fields, "\n") + "\n=end"
	default: // Go, JS, Java, C++, etc.
		var lines []string
		lines = append(lines, "/*")
		for _, f := range fields {
			lines = append(lines, " * "+f)
		}
		lines = append(lines, " */")
		return strings.Join(lines, "\n")
	}
}
