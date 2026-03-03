/**
 * Component: Contract Intent Handler
 * Block-UUID: 8a9f3c2d-1e4b-4a5c-9d6e-0f1a2b3c4d5e
 * Parent-UUID: 5b2507ac-65d7-43a6-ad10-ba8471202f2c
 * Version: 1.14.0
 * Description: Removed duplicate struct definitions (LaunchRequest, LaunchResult, etc.) and updated all function signatures to use the canonical types defined in internal/contract/models.go.
 * Language: Go
 * Created-at: 2026-03-03T18:36:19.588Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0)
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

// HandleLaunch processes a LaunchRequest and executes the appropriate workspace action.
func HandleLaunch(req LaunchRequest) (LaunchResult, error) {
	// 1. Load and Validate Contract
	meta, err := GetContract(req.ContractUUID)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("failed to load contract: %w", err)
	}

	if meta.Authcode != req.Authcode {
		return LaunchResult{}, fmt.Errorf("invalid authorization code")
	}

	// 2. Handle Alias
	switch req.Alias {
	case "terminal":
		return handleTerminalIntent(meta, req.AppOverride, "")
	case "editor":
		// If no BlockUUID, we are just opening the editor in the project root
		if req.BlockUUID == "" {
			return handleEditorRootIntent(meta, req.AppOverride)
		}
		// Otherwise, proceed to full review logic
		return handleReviewIntent(meta, req)
	case "review":
		return handleReviewIntent(meta, req)
	case "dump":
		return handleDumpIntent(meta, req)
	case "exec":
		return handleExecIntent(meta, req.Cmd)
	default:
		return LaunchResult{}, fmt.Errorf("unsupported alias: %s", req.Alias)
	}
}

// GetLaunchCapabilities returns the available aliases, apps, and commands for discovery.
func GetLaunchCapabilities() LaunchCapabilities {
	// 1. Define Aliases
	aliases := []AliasDefinition{
		{Name: "terminal", Description: "Launch an interactive terminal"},
		{Name: "editor", Description: "Open the project in an editor"},
		{Name: "review", Description: "Review staged AI code"},
		{Name: "exec", Description: "Execute a raw command"},
		{Name: "dump", Description: "Dump chat history to a filesystem tree"},
	}

	// 2. Extract Apps
	editors := make([]string, 0, len(settings.DefaultEditorTemplates))
	for k := range settings.DefaultEditorTemplates {
		editors = append(editors, k)
	}

	terminals := make([]string, 0, len(settings.DefaultTerminalTemplates))
	for k := range settings.DefaultTerminalTemplates {
		terminals = append(terminals, k)
	}

	// 3. Commands
	commands := settings.DefaultSafeSet

	return LaunchCapabilities{
		Aliases:  aliases,
		Apps:     AppDefinitions{Editors: editors, Terminals: terminals},
		Commands: commands,
	}
}

// handleTerminalIntent launches the preferred terminal in the contract's workdir.
// If cmdStr is provided, it overrides the default behavior (used by dump).
func handleTerminalIntent(meta *ContractMetadata, override string, cmdStr string) (LaunchResult, error) {
	term := meta.PreferredTerminal
	if override != "" {
		term = override
	}

	if term == "" {
		return LaunchResult{}, fmt.Errorf("no preferred terminal configured for this contract")
	}

	template, ok := settings.DefaultTerminalTemplates[term]
	if !ok {
		return LaunchResult{}, fmt.Errorf("unsupported terminal: %s", term)
	}

	// If no custom command provided, use the default workdir launch
	if cmdStr == "" {
		cmdStr = fmt.Sprintf(template, meta.Workdir)
	}

	// Increased timeout to 15s to allow for slow AppleScript/App startup
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 15, Silent: true}, meta.Workdir)
	result, err := executor.Run()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("failed to launch terminal: %w", err)
	}

	msg := fmt.Sprintf("Launched %s", term)
	if result.ExitCode != 0 {
		msg = fmt.Sprintf("Failed to launch %s: %s", term, getExitCodeDescription(result.ExitCode))
	}

	return LaunchResult{
		Success: result.ExitCode == 0,
		Message: msg,
		Alias:   "terminal",
		Workdir: meta.Workdir,
		Command: cmdStr,
	}, nil
}

// handleEditorRootIntent launches the preferred editor in the contract's workdir.
func handleEditorRootIntent(meta *ContractMetadata, override string) (LaunchResult, error) {
	editor := meta.PreferredEditor
	if override != "" {
		editor = override
	}

	if editor == "" {
		return LaunchResult{}, fmt.Errorf("no preferred editor configured for this contract")
	}

	template, ok := settings.DefaultEditorTemplates[editor]
	if !ok {
		return LaunchResult{}, fmt.Errorf("unsupported editor: %s", editor)
	}

	// Pass the absolute workdir to ensure the editor opens the correct project root
	cmdStr := fmt.Sprintf(template, meta.Workdir)
	
	// Increased timeout to 15s to allow for slow AppleScript/App startup
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 15, Silent: true}, meta.Workdir)
	result, err := executor.Run()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("failed to launch editor: %w", err)
	}

	msg := fmt.Sprintf("Launched %s in %s", editor, meta.Workdir)
	if result.ExitCode != 0 {
		msg = fmt.Sprintf("Failed to launch %s: %s", editor, getExitCodeDescription(result.ExitCode))
	}

	return LaunchResult{
		Success: result.ExitCode == 0,
		Message: msg,
		Alias:   "editor",
		Workdir: meta.Workdir,
		Command: cmdStr,
	}, nil
}

// handleReviewIntent stages AI code and launches an editor for review.
func handleReviewIntent(meta *ContractMetadata, req LaunchRequest) (LaunchResult, error) {
	// 1. Resolve Target File via Parent-UUID
	targetFile, err := ResolveFileByParentUUID(req.ParentUUID, meta.Workdir)
	if err != nil {
		return LaunchResult{}, err
	}

	// 2. Fetch and Stage Code Block
	stagedPath, err := StageCodeBlock(req.BlockUUID, targetFile)
	if err != nil {
		return LaunchResult{}, err
	}

	// 3. Resolve Editor Command
	editor := meta.PreferredReview
	if editor == "" {
		editor = meta.PreferredEditor
	}
	if req.AppOverride != "" {
		editor = req.AppOverride
	}

	if editor == "" {
		return LaunchResult{
			Success:    true,
			Message:    "Code staged successfully, but no editor is configured.",
			Alias:      "review",
			Workdir:    meta.Workdir,
			StagedPath: stagedPath,
		}, nil
	}

	template, ok := settings.DefaultEditorTemplates[editor]
	if !ok {
		return LaunchResult{}, fmt.Errorf("unsupported editor: %s", editor)
	}

	// Construct command with the staged file path
	cmdStr := fmt.Sprintf(template, stagedPath)

	// 4. Execute (with extended timeout for editors)
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 0}, meta.Workdir)
	result, err := executor.Run()
	if err != nil {
		return LaunchResult{}, fmt.Errorf("failed to launch editor: %w", err)
	}

	msg := fmt.Sprintf("Review started in %s", editor)
	if result.ExitCode != 0 {
		msg = fmt.Sprintf("Failed to start review in %s: %s", editor, getExitCodeDescription(result.ExitCode))
	}

	return LaunchResult{
		Success:    result.ExitCode == 0,
		Message:    msg,
		Alias:      "review",
		Workdir:    meta.Workdir,
		StagedPath: stagedPath,
		Command:    cmdStr,
	}, nil
}

// handleDumpIntent coordinates the dump and launches a terminal in the dump directory.
func handleDumpIntent(meta *ContractMetadata, req LaunchRequest) (LaunchResult, error) {
	// 1. Select Strategy (Default to Tree)
	var writer DumpWriter
	dumpType := req.Action
	if dumpType == "" {
		dumpType = "tree"
	}

	// Default sort mode to recency if not specified
	sortMode := req.Sort
	if sortMode == "" {
		sortMode = settings.SortRecency
	}

	switch dumpType {
	case "tree":
		writer = &TreeWriter{}
	case "merged":
		writer = &MergedWriter{}
	default:
		return LaunchResult{}, fmt.Errorf("unsupported dump type: %s", dumpType)
	}

	// 2. Execute Dump
	outputDir := GetDefaultDumpDir(meta.UUID)
	// Default to smart trim (true) for Web UI triggers
	if err := ExecuteDump(meta.UUID, writer, outputDir, false, true, dumpType, sortMode, req.DebugPatch); err != nil {
		return LaunchResult{}, fmt.Errorf("dump failed: %w", err)
	}

	// 3. Launch Terminal in the Dump Directory
	// We override the workdir for the terminal launch to the dump directory
	cmdStr := fmt.Sprintf("cd %s && clear && tree -C .", outputDir)
	
	// We use the terminal launcher but point it to the dump directory instead of the project workdir
	return handleTerminalIntent(meta, req.AppOverride, cmdStr)
}

// handleExecIntent runs a raw command in the contract context.
func handleExecIntent(meta *ContractMetadata, cmdStr string) (LaunchResult, error) {
	if cmdStr == "" {
		return LaunchResult{}, fmt.Errorf("no command provided")
	}

	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: meta.ExecTimeout}, meta.Workdir)
	result, err := executor.Run()
	if err != nil {
		return LaunchResult{}, err
	}

	msg := "Command executed successfully"
	if result.ExitCode != 0 {
		msg = fmt.Sprintf("Command failed: %s", getExitCodeDescription(result.ExitCode))
	}

	return LaunchResult{
		Success: result.ExitCode == 0,
		Message: msg,
		Alias:   "exec",
		Workdir: meta.Workdir,
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

// getExitCodeDescription maps common shell exit codes to human-readable strings.
func getExitCodeDescription(code int) string {
	switch code {
	case 1:
		return "general error"
	case 2:
		return "shell misuse"
	case 126:
		return "permission denied"
	case 127:
		return "command not found"
	case 130:
		return "interrupted"
	case 137:
		return "killed"
	case 255:
		return "exit status out of range"
	default:
		return fmt.Sprintf("exit code %d", code)
	}
}
