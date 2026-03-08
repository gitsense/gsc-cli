/**
 * Component: Contract Intent Handler
 * Block-UUID: d8db6c4f-f718-4cf6-818b-1e56d45640a8
 * Parent-UUID: 9c83fce9-a43f-4871-ad88-7b2cecd3cdf5
 * Version: 1.23.0
 * Description: Updated terminal intent handling to use the parent mapped directory for scripts and environment variables, removing hash-specific dependencies.
 * Language: Go
 * Created-at: 2026-03-06T05:23:56.122Z
 * Authors: Gemini 3 Flash (v1.0.0), ..., GLM-4.7 (v1.21.0), GLM-4.7 (v1.22.0), GLM-4.7 (v1.23.0)
 */


package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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
		return handleTerminalIntent(meta, req)
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
		return handleDumpIntent(meta, req, req.ActiveChatID)
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
// It resolves the target directory based on the code block position if provided.
func handleTerminalIntent(meta *ContractMetadata, req LaunchRequest) (LaunchResult, error) {
	term := meta.PreferredTerminal
	if req.AppOverride != "" {
		term = req.AppOverride
	}

	if term == "" {
		return LaunchResult{}, fmt.Errorf("no preferred terminal configured for this contract")
	}

	template, ok := settings.DefaultTerminalTemplates[term]
	if !ok {
		return LaunchResult{}, fmt.Errorf("unsupported terminal: %s", term)
	}

	// Check for Workspace Mode (-ws suffix)
	isWorkspace := strings.HasSuffix(term, "-ws")
	var envVars []string
	
	targetDir := meta.Workdir
	hash := req.Hash
	if hash == "" {
		hash = "latest"
	}

	// FIX: Declare workspaceDir in outer scope so it's accessible later
	var workspaceDir string

	if isWorkspace {
		// 1. Resolve Target Directory based on Position
		gscHome, _ := settings.GetGSCHome(false)
		dumpsRoot := filepath.Join(gscHome, settings.DumpsRelPath)
		workspaceDir = filepath.Join(dumpsRoot, meta.UUID, "mapped", hash)
		
		// Default to workspace root
		targetDir = workspaceDir

		if req.Position >= 0 {
			manifestPath := filepath.Join(workspaceDir, "workspace.json")
			data, err := os.ReadFile(manifestPath)
			if err == nil {
				var ws ShadowWorkspace
				if err := json.Unmarshal(data, &ws); err == nil {
					// Find the file entry matching the position
					for _, f := range ws.Files {
						if f.Position == req.Position {
							if f.Status == MappedStatusMapped {
								targetDir = filepath.Join(workspaceDir, "mapped", f.Path)
							} else if f.Path != "" {
								// Unmapped component
								targetDir = filepath.Join(workspaceDir, "unmapped", "components", f.Path)
							} else {
								// Unmapped snippet
								targetDir = filepath.Join(workspaceDir, "unmapped", "snippets")
							}
							break
						}
					}
				}
			}
		}

		// 2. Calculate mappedDir (parent of workspaceRoot)
		mappedDir := filepath.Dir(workspaceDir)

		// 3. Generate Shell Init Script in mappedDir
		if err := GenerateShellInitScript(mappedDir, req.ActiveChatID, meta.UUID, meta.Workdir, hash, targetDir); err != nil {
			return LaunchResult{}, fmt.Errorf("failed to generate shell init script: %w", err)
		}

		// 4. Prepare Environment Variables
		envVars = []string{
			fmt.Sprintf("GSC_CHAT_ID=%d", req.ActiveChatID),
			fmt.Sprintf("GSC_PROJECT_ROOT=%s", meta.Workdir),
			fmt.Sprintf("GSC_CONTRACT_UUID=%s", meta.UUID),
			fmt.Sprintf("GSC_SCRIPTS_DIR=%s", mappedDir),
		}
	}

	// Construct command string
	cmdStr := req.Cmd
	if cmdStr == "" {
		cmdStr = fmt.Sprintf(template, targetDir)
		// Inject the absolute path for the init script to avoid env var dependency at launch
		cmdStr = strings.ReplaceAll(cmdStr, "{{GSC_SCRIPTS_DIR}}", filepath.Dir(workspaceDir))
	}

	// Increased timeout to 15s to allow for slow AppleScript/App startup
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 15, Silent: true}, targetDir, envVars)
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
		Workdir: targetDir,
		Command: cmdStr,
	}, nil
}

// GenerateShellInitScript creates the .gsc-init.sh or .gsc-init.ps1 file in the workdir.
// Exported for use by the 'ws' command.
func GenerateShellInitScript(workdir string, activeChatID int64, contractUUID string, projectRoot string, hash string, targetDir string) error {
	gscHome, _ := settings.GetGSCHome(false)
	templateDir := filepath.Join(gscHome, "data", "templates", "shells", "ws")

	var templateFile, outputFile string

	switch runtime.GOOS {
	case "windows":
		templateFile = filepath.Join(templateDir, "windows", "init.ps1")
		outputFile = filepath.Join(workdir, ".gsc-init.ps1")
	default:
		templateFile = filepath.Join(templateDir, runtime.GOOS, "init.sh")
		outputFile = filepath.Join(workdir, ".gsc-init.sh")
	}

	// Read template
	content, err := os.ReadFile(templateFile)
	if err != nil {
		return fmt.Errorf("failed to read shell template: %w", err)
	}

	// Substitute Variables
	replacements := map[string]string{
		"{{GSC_CHAT_ID}}":      fmt.Sprintf("%d", activeChatID),
		"{{GSC_PROJECT_ROOT}}": projectRoot,
		"{{GSC_CONTRACT_UUID}}": contractUUID,
		"{{GSC_SCRIPTS_DIR}}":  workdir,
		"{{TARGET_DIR}}":       targetDir,
	}

	processedContent := string(content)
	for key, val := range replacements {
		processedContent = strings.ReplaceAll(processedContent, key, val)
	}

	// Write File
	if err := os.WriteFile(outputFile, []byte(processedContent), 0755); err != nil {
		return fmt.Errorf("failed to write shell init script: %w", err)
	}

	logger.Info("Shell init script generated", "path", outputFile, "os", runtime.GOOS)
	return nil
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
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 15, Silent: true}, meta.Workdir, nil)
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
	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: 0}, meta.Workdir, nil)
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
func handleDumpIntent(meta *ContractMetadata, req LaunchRequest, activeChatID int64) (LaunchResult, error) {
	// 1. Select Strategy (Default to Tree)
	var writer DumpWriter
	dumpType := req.Action
	if dumpType == "" {
		dumpType = "tree"
	}

	// Map UI action to internal dump type
	// "text" in UI usually means the merged/squashed view
	if dumpType == "text" {
		dumpType = "merged"
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

	// 2. Resolve Output Directory (Type-Aware)
	outputDir := GetDefaultDumpDir(meta.UUID, dumpType)
	
	// 3. Execute Dump
	// Note: We don't have messageID or validate flags in LaunchRequest yet, 
	// so we pass defaults (0, false).
	// We pass activeChatID here to support help file generation.
	_, err := ExecuteDump(meta.UUID, writer, outputDir, false, true, dumpType, sortMode, req.DebugPatch, 0, false, activeChatID)
	if err != nil {
		return LaunchResult{}, fmt.Errorf("dump failed: %w", err)
	}

	// 4. Launch Terminal in the Dump Directory
	// We override the workdir for the terminal launch to the dump directory
	cmdStr := fmt.Sprintf("cd %s && clear && tree -C .", outputDir)
	req.Cmd = cmdStr
	
	// We use the terminal launcher but point it to the dump directory instead of the project workdir
	return handleTerminalIntent(meta, req)
}

// handleExecIntent runs a raw command in the contract context.
func handleExecIntent(meta *ContractMetadata, cmdStr string) (LaunchResult, error) {
	if cmdStr == "" {
		return LaunchResult{}, fmt.Errorf("no command provided")
	}

	executor := exec.NewExecutor(cmdStr, exec.ExecFlags{TimeoutSeconds: meta.ExecTimeout}, meta.Workdir, nil)
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
