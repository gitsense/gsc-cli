/**
 * Component: Import Git Command Logic
 * Block-UUID: b143c720-4f92-4cda-86d2-f7ce7f003ae4
 * Parent-UUID: d3d15358-0a68-4ae4-87cd-b468755545fc
 * Version: 1.33.0
 * Description: Added integration with centralized gitignore service to ensure import-specific patterns are added to .gitsense/.gitignore after successful import operations.
 * Language: Go
 * Created-at: 2026-05-31T14:17:55.503Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.3.1), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.5.1), GLM-4.7 (v1.5.2), GLM-4.7 (v1.5.3), GLM-4.7 (v1.5.4), GLM-4.7 (v1.6.0), GLM-4.7 (v1.7.0), GLM-4.7 (v1.8.0), GLM-4.7 (v1.9.0), GLM-4.7 (v1.10.0), GLM-4.7 (v1.11.0), GLM-4.7 (v1.12.0), GLM-4.7 (v1.13.0), GLM-4.7 (v1.14.0), GLM-4.7 (v1.15.0), GLM-4.7 (v1.16.0), GLM-4.7 (v1.17.0), GLM-4.7 (v1.18.0), GLM-4.7 (v1.19.0), GLM-4.7 (v1.20.0), Gemini 2.5 Flash Lite (v1.21.0), GLM-4.7 (v1.22.0), GLM-4.7 (v1.23.0), GLM-4.7 (v1.23.1), GLM-4.7 (v1.24.0), GLM-4.7 (v1.25.0), GLM-4.7 (v1.26.0), GLM-4.7 (v1.27.0), GLM-4.7 (v1.28.0), GLM-4.7 (v1.29.0), GLM-4.7 (v1.30.0), Gemini 2.5 Flash Lite (v1.31.0), DeepSeek V4 Pro (v1.32.0), GLM-4.7 (v1.33.0)
 */


package importgit

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/docker"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/internal/gitignore"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

// runGit executes the git import workflow
func runGit(cmd *cobra.Command, args []string) error {
	var gitPath string
	var err error

	// Resolve GSC Home early for shadow operations
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	// Phase 0: Handle --delete-shadow
	if flagDeleteShadow {
		return runDeleteShadow(gscHome, gitPath, flagOwner, flagRepo, flagBranch, flagForce)
	}

	// Phase 0.5: Handle --status (early exit before any database interaction)
	if flagStatus {
		return handleStatus(gscHome, gitPath, flagOwner, flagRepo, flagBranch)
	}

	// Phase 0.6: Flag Validation for Rebuild/Update
	if flagRebuild && flagUpdate {
		cmd.SilenceUsage = true
		return errors.New("flags --rebuild and --update are mutually exclusive")
	}
	if flagRebuild && flagResume {
		cmd.SilenceUsage = true
		return errors.New("flags --rebuild and --resume are mutually exclusive")
	}

	// Phase 1: Resolve Paths & Load State
	
	// Resolve Git Path FIRST to ensure --update uses the correct repo
	if flagPath != "" {
		gitPath, err = filepath.Abs(flagPath)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("invalid git path: %w", err)
		}
	} else {
		gitPath, err = git.FindGitRoot()
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to find git repository: %w (specify --path if not in CWD)", err)
		}
	}

	// Handle --resume validation
	// v1.28.0: Updated to read rebuild state from import-git.json instead of separate rebuild-state.json
	if flagResume {
		importState, err := LoadState(gitPath)
		if err != nil {
			cmd.SilenceUsage = true
			return errors.New("no rebuild state found; run without --resume to start fresh")
		}
		// Find any branch with an active rebuild in progress
		foundBranch := ""
		for branch, bs := range importState.Branches {
			if bs.Rebuild != nil {
				foundBranch = branch
				break
			}
		}
		if foundBranch == "" {
			cmd.SilenceUsage = true
			return errors.New("no rebuild state found; run without --resume to start fresh")
		}
		if flagOwner == "" {
			flagOwner = importState.Owner
		}
		if flagRepo == "" {
			flagRepo = importState.Repo
		}
		if flagBranch == "" {
			flagBranch = foundBranch
		}
	}

	// Handle --update logic now that gitPath is resolved
	if flagUpdate {
		state, err := LoadState(gitPath)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to load import state: %w (use 'gsc app import git' without --update for first import)", err)
		}

		// If --branch is missing, try to get current branch name
		if flagBranch == "" {
			cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			cmdBranch.Dir = gitPath
			output, err := cmdBranch.Output()
			if err == nil {
				flagBranch = strings.TrimSpace(string(output))
			} else {
				cmd.SilenceUsage = true
				return fmt.Errorf("--branch is required for --update")
			}
		}

		if branchState, ok := state.Branches[flagBranch]; ok {
			if flagOwner == "" {
				flagOwner = state.Owner
			}
			if flagRepo == "" {
				flagRepo = state.Repo
			}
			// Phase 2: Validate shadow state if updating a shadow import
			if branchState.Shadow {
				if !ShadowExists(branchState.ShadowPath) {
					logger.Warning("Shadow repo missing, falling back to full import", "path", branchState.ShadowPath)
				} else {
					// Shadow exists, enforce shadow mode to prevent prompting
					flagShadow = true
				}
			}
		} else {
			cmd.SilenceUsage = true
			return fmt.Errorf("no import state found for branch '%s'", flagBranch)
		}
	} else {
		// Infer flags for rebuild from existing import state
		if flagRebuild && !flagResume {
			state, err := LoadState(gitPath)
			if err == nil {
				if flagOwner == "" {
					flagOwner = state.Owner
				}
				if flagRepo == "" {
					flagRepo = state.Repo
				}
			}
		}

		// Auto-detect branch if not provided for new imports
		if flagBranch == "" {
			cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			cmdBranch.Dir = gitPath
			output, err := cmdBranch.Output()
			if err == nil {
				flagBranch = strings.TrimSpace(string(output))
				// Check if we're in detached HEAD state
				if flagBranch == "HEAD" {
					cmd.SilenceUsage = true
					return fmt.Errorf("detached HEAD state detected. Please specify --branch explicitly or checkout a branch.")
				}
			} else {
				cmd.SilenceUsage = true
				return fmt.Errorf("failed to auto-detect branch. Please specify --branch explicitly.")
			}
		}
		
		// Validate required flags for new imports
		if flagRepo == "" || flagOwner == "" {
			cmd.SilenceUsage = true
			return fmt.Errorf("--repo and --owner are required for new imports")
		}
	}

	// Resolve DB Path
	dbPath := flagDBPath
	if dbPath == "" {
		// Priority: Docker Context -> Env Vars -> Default
		dockerCtx, err := docker.LoadContext()
		if err == nil && dockerCtx != nil {
			dbPath = filepath.Join(dockerCtx.DataHostPath, "chats.sqlite3")
		} else {
			dbPath = settings.GetChatDatabasePath(gscHome)
		}
	}

	// Resolve GSCB CLI Path
	gscbCliPath := filepath.Join(gscHome, "node_modules", "@gitsense", "gscb-cli", "dist", "bin", "gscb.js")
	if _, err := os.Stat(gscbCliPath); os.IsNotExist(err) {
		cmd.SilenceUsage = true
		return fmt.Errorf("gscb-cli not found at %s (run 'npm install' in GSC_HOME)", gscbCliPath)
	}

	// Phase 2: Validation
	// 1. Validate Git Repo
	if _, err := os.Stat(filepath.Join(gitPath, ".git")); os.IsNotExist(err) {
		cmd.SilenceUsage = true
		return fmt.Errorf("not a git repository: %s", gitPath)
	}

	// 2. Validate Branch
	if !checkBranchExists(gitPath, flagBranch) {
		cmd.SilenceUsage = true
		return fmt.Errorf("branch '%s' does not exist in %s", flagBranch, gitPath)
	}

	// Phase 2.5: Check for existing imports in GitSense Chat Application (new imports only)
	if !flagUpdate && !flagRebuild && !flagResume {
		dbConn, err := db.OpenDB(dbPath)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to open database for validation: %w", err)
		}
		defer db.CloseDB(dbConn)

		if err := checkExistingChatImport(cmd, dbConn, gitPath, flagOwner, flagRepo, flagBranch, dbPath); err != nil {
			return err
		}
	}

	// 3. Validate DB Path (create parent dirs if needed)
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// ==========================================
	// REBUILD WORKFLOW
	// ==========================================
	if flagRebuild || flagResume {
		return runRebuildWorkflow(cmd, gitPath, gscHome, dbPath)
	}

	// ==========================================
	// STANDARD IMPORT / UPDATE WORKFLOW
	// ==========================================

	// Phase 3: Mode Selection
	// Shadow mode is now enforced for all new imports
	useShadow := true

	// Initialize ProgressUI ONCE before any work starts
	// This ensures GetDuration() includes both shadow and import phases
	progressUI := NewProgressUI()

	// Phase 4: Pre-flight Summary & Status Check
	shadowPath := ShadowPath(gscHome, flagOwner, flagRepo, flagBranch)
	shadowExists := ShadowExists(shadowPath)
	
	// Get commit metadata for shadow status display
	var sourceMsg, sourceTime, shadowMsg, shadowTime string
	if shadowExists {
		var err error
		sourceMsg, _, sourceTime, err = getSourceCommitMeta(gitPath)
		if err != nil {
			logger.Warning("Failed to get source commit metadata", "error", err)
			sourceMsg = "unknown"
			sourceTime = ""
		}
		
		shadowMsg, _, shadowTime, err = getSourceCommitMeta(shadowPath)
		if err != nil {
			logger.Warning("Failed to get shadow commit metadata", "error", err)
			shadowMsg = "unknown"
			shadowTime = ""
		}
	}
	
	// Determine action
	action := "New Import"
	if flagUpdate {
		action = "Update (incremental)"
	}
	
	// Phase 4.5: Prompt user if shadow exists
	// Only prompt if not using --update or --force flags
	if shadowExists && !flagUpdate && !flagForce {
		fmt.Println("\nA shadow repository already exists for this branch.")
		fmt.Printf("    Shadow snapshot: \"%s\" (%s)\n", shadowMsg, shadowTime)
		fmt.Printf("    Source commit:    \"%s\" (%s)\n", sourceMsg, sourceTime)
		fmt.Println()
		
		choice := ""
		prompt := &survey.Select{
			Message: "How would you like to proceed?",
			Options: []string{"Update existing", "Cancel"},
			Default: "Update existing",
		}
		if err := survey.AskOne(prompt, &choice); err != nil {
			return err
		}
		
		if choice == "Cancel" {
			fmt.Println("Cancelled.")
			return nil
		}
		
		// User chose "Update existing"
		action = "Update (incremental)"
	}
	
	// Phase 4.6: Check for uncommitted changes BEFORE creating/updating shadow
	// This check applies to both new imports and updates to ensure shadow repo
	// always matches a clean commit state in the source repository
	if useShadow {
		hasUncommitted, err := git.HasUncommittedChanges(gitPath)
		if err != nil {
			cmd.SilenceUsage = true
			return fmt.Errorf("failed to check for uncommitted changes: %w", err)
		}
		
		if hasUncommitted {
			cmd.SilenceUsage = true
			return fmt.Errorf("source repository has uncommitted changes. Please commit or stash your changes before creating or updating the shadow repository.")
		}
	}
	
	if !flagForce {
		fmt.Println("\nReady to import:")
		fmt.Printf("  Source:      %s\n", gitPath)
		fmt.Printf("  Branch:      %s\n", flagBranch)
		fmt.Printf("  Target DB:   %s\n", dbPath)
		
		// Display mode based on actual useShadow value
		if useShadow {
			fmt.Printf("  Mode:        Shadow (single-commit snapshot)\n")
			fmt.Printf("  Shadow Path: %s\n", shadowPath)
		} else {
			fmt.Printf("  Mode:        Full (preserves history)\n")
		}
		
		fmt.Printf("  Action:      %s\n", action)
		
		if shadowExists {
			fmt.Println()
			
			// Check if we have state for this branch
			state, err := LoadState(gitPath)
			hasState := err == nil
			
			if hasState {
				if branchState, ok := state.Branches[flagBranch]; ok && branchState.Shadow {
					// We have state and it's a shadow import
					fmt.Println("  Shadow Status:")
					fmt.Printf("    Shadow snapshot: \"%s\" (%s)\n", shadowMsg, shadowTime)
					fmt.Printf("    Source commit:   \"%s\" (%s)\n", sourceMsg, sourceTime)
				} else {
					// We have state but it's not marked as shadow
					fmt.Println("  ⚠️  Shadow repository exists but import state indicates full mode")
				}
			} else {
				// No state file found
				fmt.Println("  ⚠️  Shadow repository exists but no import state found")
				fmt.Println("  Shadow Status:")
				fmt.Printf("    Shadow snapshot: \"%s\" (%s)\n", shadowMsg, shadowTime)
				fmt.Printf("    Source commit:   \"%s\" (%s)\n", sourceMsg, sourceTime)
			}
		}
		
		fmt.Println()
		if useShadow {
			fmt.Println("This will copy files to the shadow repository.")
		} else {
			fmt.Println("This will import the full repository history.")
		}
		
		// Show delete command if shadow exists
		if shadowExists {
			fmt.Println()
			fmt.Printf("To delete and recreate: gsc app import git --delete-shadow --owner %s --repo %s --branch %s\n", flagOwner, flagRepo, flagBranch)
		}
		
		fmt.Println()
		
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Import cancelled.")
			return nil
		}
	}

	// Phase 5: Shadow Orchestration
	// Note: This now uses the optimized shadow.go v2.0.0 functions
	// which include APFS clonefile support and git update-index optimization
	if useShadow {
		// shadowPath is already set above
		
		// Check if shadow exists to determine which phase to start
		if ShadowExists(shadowPath) {
			// Use update phase for existing shadows
			progressUI.StartShadowUpdatePhase()
			logger.Info("Updating shadow repository...", "path", shadowPath)
			err = UpdateShadow(shadowPath, gitPath, progressUI.ShadowProgressFn())
		} else {
			// Use create phase for new shadows
			progressUI.StartShadowPhase()
			logger.Info("Creating shadow repository...", "path", shadowPath)
			// Pass branch parameter to ensure shadow repo has correct branch name
			err = CreateShadow(shadowPath, gitPath, flagBranch, progressUI.ShadowProgressFn())
		}

		if err != nil {
			if errors.Is(err, ErrNoChanges) {
				progressUI.Cleanup()
				fmt.Println("Shadow repository is already up to date. No changes detected since last import.")
				return nil
			}
			cmd.SilenceUsage = true
			return fmt.Errorf("shadow repo operation failed: %w", err)
		}
	}

	// Phase 6: Execute
	// Determine import path (source or shadow)
	importPath := gitPath
	if useShadow {
		importPath = shadowPath
	}

	// Build args
	cmdArgs := []string{
		gscbCliPath,
		"import", "git",
		importPath,
		flagOwner,
		flagRepo,
		flagBranch,
		"--db-path", dbPath,
	}

	// Phase 2: Wire --display-repo-path if using shadow
	if useShadow {
		cmdArgs = append(cmdArgs, "--display-repo-path", gitPath)
	}

	if flagMaxSize > 0 {
		cmdArgs = append(cmdArgs, "--max-size", fmt.Sprintf("%d", flagMaxSize))
	}
	if flagInclude != "" {
		cmdArgs = append(cmdArgs, "--include", flagInclude)
	}
	if flagExclude != "" {
		cmdArgs = append(cmdArgs, "--exclude", flagExclude)
	}
	if flagIncludeBinary {
		cmdArgs = append(cmdArgs, "--include-binary")
	}
	if flagVerbose {
		cmdArgs = append(cmdArgs, "--verbose")
	}

	// Spawn process
	execCmd := exec.Command("node", cmdArgs...)
	
	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	execCmd.Stderr = os.Stderr // Send stderr directly to terminal

	logger.Debug("Starting gscb-cli", "args", cmdArgs)
	logger.Debug("Executing gscb-cli command", "command", strings.Join(append([]string{"node"}, cmdArgs...), " "))

	if err := execCmd.Start(); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to start gscb-cli: %w", err)
	}

	// Parse Stream (REUSE existing progressUI)
	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		event, err := ParseLine(line)
		if err != nil {
			logger.Warning("Failed to parse NDJSON line", "line", line, "error", err)
			continue
		}
		progressUI.Update(event)
	}

	if err := scanner.Err(); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("error reading gscb-cli output: %w", err)
	}

	// Wait for completion
	if err := execCmd.Wait(); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("gscb-cli process failed: %w", err)
	}

	// Phase 6.5: Update .gitignore with import-specific patterns
	gitsenseDir := filepath.Join(gitPath, settings.GitSenseDir)
	if err := gitignore.EnsureUpdated(gitsenseDir, gitignore.Registration{
		Source: gitignore.SourceImport,
		Patterns: []string{"import-git.json", ".import.lock", "backups/"},
		WarnFn: func(msg string) {
			logger.Info("Gitignore updated", "message", msg)
		},
	}); err != nil {
		logger.Warning("Failed to update .gitignore with import patterns", "error", err)
		// Non-fatal error, continue
	}

	// Phase 6.6: Get RefChatID and GroupID for state persistence
	// We need the ref_chat_id from the progressUI
	refChatID := progressUI.GetRefChatID()
	if refChatID == 0 {
		cmd.SilenceUsage = true
		return fmt.Errorf("import completed but no ref_chat_id was received")
	}

	// We need to query the database to get the group_id associated with the refChatID
	var groupID int64
	dbConn, err := db.OpenDB(dbPath)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to open database to fetch group_id: %w", err)
	}
	defer db.CloseDB(dbConn)

	err = dbConn.QueryRow("SELECT group_id FROM chats WHERE id = ? AND deleted = 0", refChatID).Scan(&groupID)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to query group_id for ref %d: %w", refChatID, err)
	}

	// Phase 7: Save State
	importFlags := ImportFlags{
		MaxSize:       flagMaxSize,
		Include:       flagInclude,
		Exclude:       flagExclude,
		IncludeBinary: flagIncludeBinary,
	}

	// Phase 2: Pass shadow info to SaveState
	// Phase 3: Pass groupID to SaveState
	if err := SaveState(gitPath, flagOwner, flagRepo, flagBranch, refChatID, groupID, importFlags, useShadow, shadowPath); err != nil {
		logger.Warning("Failed to save import state", "error", err)
	}

	// Phase 8: Post-Import Summary
	// Cleanup UI before printing summary to avoid extra newlines
	progressUI.Cleanup()

	stateFileRelPath := filepath.Join(".gitsense", stateFileName)
	ignored, _ := CheckGitIgnore(gitPath, stateFileRelPath)
	
	if useShadow {
		progressUI.PrintShadowFinalSummary(flagOwner, flagRepo, flagBranch, dbPath, filepath.Join(gitPath, stateFileRelPath), shadowPath, progressUI.GetDuration())
	} else {
		progressUI.PrintFinalSummaryWithWarning(flagOwner, flagRepo, flagBranch, dbPath, filepath.Join(gitPath, stateFileRelPath), progressUI.GetDuration(), !ignored)
	}

	return nil
}

// handleStatus displays the shadow repository status without opening a database connection
// This is called early in the workflow to prevent interactive prompts and database access
func handleStatus(gscHome, gitPath, owner, repo, branch string) error {
	// Resolve minimal context needed for status display
	// Priority: flags -> state file -> auto-detect
	if owner == "" || repo == "" || branch == "" {
		// Try to load from state file
		if gitPath == "" {
			// Auto-detect git path if not provided
			var err error
			gitPath, err = git.FindGitRoot()
			if err != nil {
				return fmt.Errorf("failed to find git repository: %w (specify --path if not in CWD)", err)
			}
		}
		
		state, err := LoadState(gitPath)
		if err == nil {
			if owner == "" {
				owner = state.Owner
			}
			if repo == "" {
				repo = state.Repo
			}
			if branch == "" {
				// If branch not specified, use the first branch in state
				for b := range state.Branches {
					branch = b
					break
				}
			}
		}
	}
	
	// If still missing required info, try auto-detect
	if gitPath == "" {
		var err error
		gitPath, err = git.FindGitRoot()
		if err != nil {
			return fmt.Errorf("failed to find git repository: %w (specify --path if not in CWD)", err)
		}
	}
	
	if branch == "" {
		cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmdBranch.Dir = gitPath
		output, err := cmdBranch.Output()
		if err == nil {
			branch = strings.TrimSpace(string(output))
		}
	}
	
	// Validate we have enough info to show status
	if owner == "" || repo == "" || branch == "" {
		return fmt.Errorf("insufficient information to show status. Please specify --owner, --repo, and --branch, or run from within a git repository with an existing import state file.")
	}
	
	// Display status
	shadowPath := ShadowPath(gscHome, owner, repo, branch)
	shadowExists := ShadowExists(shadowPath)
	
	fmt.Println("Shadow Repository Status:")
	fmt.Printf("  Repository: %s/%s\n", owner, repo)
	fmt.Printf("  Branch: %s\n", branch)
	fmt.Printf("  Shadow Path: %s\n", shadowPath)
	
	if shadowExists {
		fmt.Println("  Status: Found")
		
		// Get commit metadata
		var sourceMsg, sourceTime, shadowMsg, shadowTime string
		var err error
		
		sourceMsg, _, sourceTime, err = getSourceCommitMeta(gitPath)
		if err != nil {
			logger.Warning("Failed to get source commit metadata", "error", err)
			sourceMsg = "unknown"
			sourceTime = ""
		}
		
		shadowMsg, _, shadowTime, err = getSourceCommitMeta(shadowPath)
		if err != nil {
			logger.Warning("Failed to get shadow commit metadata", "error", err)
			shadowMsg = "unknown"
			shadowTime = ""
		}
		
		fmt.Printf("    Shadow snapshot: \"%s\" (%s)\n", shadowMsg, shadowTime)
		fmt.Printf("    Source commit:    \"%s\" (%s)\n", sourceMsg, sourceTime)
		
		// Show shadow size
		size, err := ShadowSize(shadowPath)
		if err == nil {
			fmt.Printf("    Size: %s\n", formatBytes(size))
		}
	} else {
		fmt.Println("  Status: Not found")
	}
	
	return nil
}
