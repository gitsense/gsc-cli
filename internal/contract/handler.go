/**
 * Component: Contract Intent Handler
 * Block-UUID: 8d3156b4-c99c-4a6f-8cd3-b4993e287902
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements the core logic for handling intent-based requests from the Web UI. Includes file resolution via ripgrep, AI code staging, and context-aware launching of terminals and editors.
 * Language: Go
 * Created-at: 2026-03-01T18:26:49.477Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/exec"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// HandleReview processes a ReviewRequest and executes the appropriate workspace action.
func HandleReview(req ReviewRequest) (ReviewResult, error) {
	// 1. Load and Validate Contract
	meta, err := GetContract(req.ContractUUID)
	if err != nil {
		return ReviewResult{}, fmt.Errorf("failed to load contract: %w", err)
	}

	if meta.Authcode != req.Authcode {
		return ReviewResult{}, fmt.Errorf("invalid authorization code")
	}

	// 2. Handle Intent
	switch req.Intent {
	case "terminal":
		return handleTerminalIntent(meta, req.TerminalOverride)
	case "editor":
		// If no BlockUUID, we are just opening the editor in the project root
		if req.BlockUUID == "" {
			return handleEditorRootIntent(meta, req.EditorOverride)
		}
		// Otherwise, proceed to full review logic
		return handleReviewIntent(meta, req)
	case "review":
		return handleReviewIntent(meta, req)
	case "exec":
		return handleExecIntent(meta, req.Cmd)
	default:
		return ReviewResult{}, fmt.Errorf("unsupported intent: %s", req.Intent)
	}
}

// handleTerminalIntent launches the preferred terminal in the contract's workdir.
func handleTerminalIntent(meta *ContractMetadata, override string) (ReviewResult, error) {
	term := meta.PreferredTerminal
	if override != "" {
		term = override
	}

	if term == "" {
		return ReviewResult{}, fmt.Errorf("no preferred terminal configured for this contract")
	}

	template, ok := settings.DefaultTerminalTemplates[term]
	if !ok {
		return ReviewResult{}, fmt.Errorf("unsupported terminal: %s", term)
	}

	// Terminals usually open in a directory, so we pass "." as the path
	cmdStr := fmt.Sprintf(template, ".")
	
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 5}, meta.Workdir)
	_, err := executor.Run()
	if err != nil {
		return ReviewResult{}, fmt.Errorf("failed to launch terminal: %w", err)
	}

	return ReviewResult{
		Success: true,
		Message: fmt.Sprintf("Launched %s in %s", term, meta.Workdir),
		Command: cmdStr,
	}, nil
}

// handleEditorRootIntent launches the preferred editor in the contract's workdir.
func handleEditorRootIntent(meta *ContractMetadata, override string) (ReviewResult, error) {
	editor := meta.PreferredEditor
	if override != "" {
		editor = override
	}

	if editor == "" {
		return ReviewResult{}, fmt.Errorf("no preferred editor configured for this contract")
	}

	template, ok := settings.DefaultEditorTemplates[editor]
	if !ok {
		return ReviewResult{}, fmt.Errorf("unsupported editor: %s", editor)
	}

	// Opening the root directory
	cmdStr := fmt.Sprintf(template, ".")
	
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 5}, meta.Workdir)
	_, err := executor.Run()
	if err != nil {
		return ReviewResult{}, fmt.Errorf("failed to launch editor: %w", err)
	}

	return ReviewResult{
		Success: true,
		Message: fmt.Sprintf("Launched %s in %s", editor, meta.Workdir),
		Command: cmdStr,
	}, nil
}

// handleReviewIntent stages AI code and launches an editor for review.
func handleReviewIntent(meta *ContractMetadata, req ReviewRequest) (ReviewResult, error) {
	// 1. Resolve Target File via Parent-UUID
	targetFile, err := ResolveFileByParentUUID(req.ParentUUID, meta.Workdir)
	if err != nil {
		return ReviewResult{}, err
	}

	// 2. Fetch and Stage Code Block
	stagedPath, err := StageCodeBlock(req.BlockUUID, targetFile)
	if err != nil {
		return ReviewResult{}, err
	}

	// 3. Resolve Editor Command
	editor := meta.PreferredEditor
	if req.EditorOverride != "" {
		editor = req.EditorOverride
	}

	if editor == "" {
		return ReviewResult{
			Success:    true,
			Message:    "Code staged successfully, but no editor is configured.",
			StagedPath: stagedPath,
		}, nil
	}

	template, ok := settings.DefaultEditorTemplates[editor]
	if !ok {
		return ReviewResult{}, fmt.Errorf("unsupported editor: %s", editor)
	}

	// Construct command with the staged file path
	cmdStr := fmt.Sprintf(template, stagedPath)

	// 4. Execute (with extended timeout for editors)
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 0}, meta.Workdir)
	_, err = executor.Run()
	if err != nil {
		return ReviewResult{}, fmt.Errorf("failed to launch editor: %w", err)
	}

	return ReviewResult{
		Success:    true,
		Message:    fmt.Sprintf("Review started in %s", editor),
		StagedPath: stagedPath,
		Command:    cmdStr,
	}, nil
}

// handleExecIntent runs a raw command in the contract context.
func handleExecIntent(meta *ContractMetadata, cmdStr string) (ReviewResult, error) {
	if cmdStr == "" {
		return ReviewResult{}, fmt.Errorf("no command provided")
	}

	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: meta.ExecTimeout}, meta.Workdir)
	result, err := executor.Run()
	if err != nil {
		return ReviewResult{}, err
	}

	msg := "Command executed successfully"
	if result.ExitCode != 0 {
		msg = fmt.Sprintf("Command failed with exit code %d", result.ExitCode)
	}

	return ReviewResult{
		Success: result.ExitCode == 0,
		Message: msg,
		Command: cmdStr,
	}, nil
}

// ResolveFileByParentUUID uses ripgrep to find the file containing the Parent-UUID.
func ResolveFileByParentUUID(parentUUID string, workdir string) (string, error) {
	if parentUUID == "" || parentUUID == "N/A" {
		return "", nil // New file case
	}

	logger.Debug("Resolving file by Parent-UUID", "uuid", parentUUID, "workdir", workdir)

	// Use existing ripgrep executor
	options := manifest.RgOptions{
		Pattern:       parentUUID,
		CaseSensitive: true,
	}

	// We need to run this in the workdir
	oldCwd, _ := os.Getwd()
	os.Chdir(workdir)
	defer os.Chdir(oldCwd)

	matches, err := manifest.ExecuteRipgrep(options)
	if err != nil {
		return "", fmt.Errorf("ripgrep failed: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("Parent-UUID '%s' not found in workspace. The file may have been moved or deleted.", parentUUID)
	}

	// Check for multiple files
	uniqueFiles := make(map[string]bool)
	for _, m := range matches {
		uniqueFiles[m.FilePath] = true
	}

	if len(uniqueFiles) > 1 {
		var files []string
		for f := range uniqueFiles {
			files = append(files, f)
		}
		return "", fmt.Errorf("ambiguous match: Parent-UUID found in multiple files: %s", strings.Join(files, ", "))
	}

	// Return the single match
	for f := range uniqueFiles {
		return f, nil
	}

	return "", fmt.Errorf("unexpected error during file resolution")
}

// StageCodeBlock fetches the AI code from the database and writes it to a temporary review file.
func StageCodeBlock(blockUUID string, targetFile string) (string, error) {
	if blockUUID == "" {
		return "", fmt.Errorf("Block-UUID is required for staging")
	}

	// 1. Fetch Code from DB
	gscHome, _ := settings.GetGSCHome(false)
	sqliteDB, err := db.OpenDB(settings.GetChatDatabasePath(gscHome))
	if err != nil {
		return "", err
	}
	defer sqliteDB.Close()

	// Query for the message containing the Block-UUID
	var content string
	query := `SELECT message FROM messages WHERE message LIKE ? AND deleted = 0 LIMIT 1`
	err = sqliteDB.QueryRow(query, "%"+blockUUID+"%").Scan(&content)
	if err != nil {
		return "", fmt.Errorf("failed to find code block with UUID %s in chat history", blockUUID)
	}

	// 2. Extract Code from Markdown
	code, ext := extractCodeBlock(content, blockUUID)
	if code == "" {
		return "", fmt.Errorf("failed to extract code from message content")
	}

	// 3. Prepare Staging Directory
	stagingDir, err := settings.GetReviewStagingDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return "", err
	}

	// 4. Write to Temp File
	fileName := fmt.Sprintf("review_%s%s", blockUUID[:8], ext)
	if targetFile != "" {
		// If we have a target file, try to use its name for better context in the editor
		base := filepath.Base(targetFile)
		fileName = fmt.Sprintf("review_%s_%s", blockUUID[:8], base)
	}

	stagedPath := filepath.Join(stagingDir, fileName)
	if err := os.WriteFile(stagedPath, []byte(code), 0644); err != nil {
		return "", fmt.Errorf("failed to write staged file: %w", err)
	}

	logger.Info("Code staged for review", "path", stagedPath)
	return stagedPath, nil
}

// extractCodeBlock parses the markdown to find the code block containing the UUID.
func extractCodeBlock(markdown string, uuid string) (string, string) {
	// This regex looks for code blocks that contain the UUID in their header
	// It captures the language and the content
	re := regexp.MustCompile("(?s)```([a-zA-Z0-9]*).*?" + regexp.QuoteMeta(uuid) + ".*?\n\n(.*?)\n```")
	matches := re.FindStringSubmatch(markdown)

	if len(matches) < 3 {
		return "", ""
	}

	lang := matches[1]
	code := matches[2]

	// Map language to extension
	ext := ".txt"
	switch strings.ToLower(lang) {
	case "go":
		ext = ".go"
	case "javascript", "js":
		ext = ".js"
	case "typescript", "ts":
		ext = ".ts"
	case "python", "py":
		ext = ".py"
	case "rust", "rs":
		ext = ".rs"
	case "markdown", "md":
		ext = ".md"
	}

	return code, ext
}
