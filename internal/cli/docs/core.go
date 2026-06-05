/**
 * Component: Docs Core Logic
 * Block-UUID: c31fc6b9-6b7b-490c-b909-1ede73768010
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Core logic for reading and displaying markdown documentation files from the embedded TemplateFS. Handles file access, error management, and content output.
 * Language: Go
 * Created-at: 2026-05-30T02:56:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// printDoc reads a markdown file from the embedded TemplateFS and prints it to stdout.
// The topic parameter should be the filename without the .md extension (e.g., "init", "about").
func printDoc(topic string) error {
	// Construct the path to the markdown file in the embedded filesystem
	filePath := fmt.Sprintf("templates/docs/%s.md", topic)

	// Read the file from the embedded TemplateFS
	content, err := settings.TemplateFS.ReadFile(filePath)
	if err != nil {
		logger.Error("Failed to read documentation file", "topic", topic, "error", err)
		return fmt.Errorf("documentation not found: %s (use 'gsc docs init' to see available topics)", topic)
	}

	// Print the content to stdout
	fmt.Fprint(os.Stdout, string(content))

	return nil
}
