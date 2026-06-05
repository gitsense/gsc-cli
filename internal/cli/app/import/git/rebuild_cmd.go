/*
 * Component: Import Git Rebuild Command
 * Block-UUID: bfa9da1c-cdaf-4a87-b125-d754008f514c
 * Parent-UUID: e6be829f-3b36-4f82-acc2-3adbae9cab14
 * Version: 1.0.0
 * Description: Extracted rebuild workflow logic from command.go to separate the rebuild orchestration from the standard import flow.
 * Language: Go
 * Created-at: 2026-05-17T15:10:01.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package importgit

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

// runRebuildWorkflow handles the --rebuild and --resume workflow
func runRebuildWorkflow(cmd *cobra.Command, gitPath, gscHome, dbPath string) error {
	// Open DB connection
	dbConn, err := db.OpenDB(dbPath)
	if err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.CloseDB(dbConn)

	// Resolve RefChatID for confirmation/status
	// We need this to show the user what we are about to delete
	var refChatID int64
	var groupID int64
	
	// Try to get GroupID first
	groupID, err = db.GetGroupID(dbConn, flagOwner, flagRepo)
	if err != nil {
		// If repo doesn't exist, we can't rebuild it (unless it's a resume from a deleted state?)
		// For now, assume we need the repo to exist to rebuild it.
		cmd.SilenceUsage = true
		return fmt.Errorf("repository '%s/%s' not found in database. Cannot rebuild.", flagOwner, flagRepo)
	}

	// Try to get RefChatID
	refChatID, err = db.GetRefChatID(dbConn, groupID, flagBranch)
	if err != nil {
		// Branch might not exist yet if this is a fresh rebuild attempt that failed early?
		// Or if we are resuming after a delete.
		// We'll proceed but warn if we can't find it.
		logger.Warning("Could not resolve RefChatID for branch", "branch", flagBranch, "error", err)
	}

	shadowPath := ShadowPath(gscHome, flagOwner, flagRepo, flagBranch)

	// Confirmation
	if flagRebuild && !flagForce {
		fmt.Println("\n⚠️  REBUILD MODE")
		fmt.Println()
		fmt.Println("This operation will completely refresh the branch data to fix issues with")
		fmt.Println("deleted or renamed files that the standard update process cannot handle.")
		fmt.Println()
		fmt.Println("The following steps will be performed:")
		fmt.Println("  1. Dump existing analysis data (if any)")
		fmt.Println("  2. Delete the shadow repository")
		fmt.Println("  3. Soft delete the branch from the database")
		fmt.Println("  4. Perform a fresh import")
		fmt.Println("  5. Restore analysis data")
		fmt.Println()
		fmt.Println("⚠️  WARNING: This will soft delete all existing chat history for this branch.")
		fmt.Println("  Data can be recovered until a permanent cleanup is run.")
		fmt.Println()
		fmt.Println("⚠️  CRITICAL WARNING: Chat ID Changes")
		fmt.Println()
		fmt.Println("This rebuild will create NEW chat IDs for all files in this branch.")
		fmt.Println("Any manifest files or brains that reference the old chat IDs will no longer work.")
		fmt.Println()
		fmt.Println("Impact:")
		fmt.Println("  • Manifests using chat IDs will need to be recreated")
		fmt.Println("  • Brains referencing specific chat IDs will break")
		fmt.Println("  • Saved contexts with chat ID references will be invalid")
		fmt.Println()
		fmt.Println("Note: Old chat IDs are soft-deleted and can be recovered if needed,")
		fmt.Println("      but they will not be used by the new import.")
		fmt.Println()
		fmt.Printf("Repository:  %s/%s\n", flagOwner, flagRepo)
		fmt.Printf("Branch:      %s\n", flagBranch)
		fmt.Printf("Shadow Path: %s\n", shadowPath)
		fmt.Println()

		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed with rebuild?",
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
	} else if flagRebuild && flagForce {
		fmt.Println("\n⚠️  REBUILD MODE (Forced)")
		fmt.Println()
		fmt.Println("Performing full rebuild to fix deleted/renamed file issues...")
		fmt.Printf("  Repository: %s/%s\n", flagOwner, flagRepo)
		fmt.Printf("  Branch:     %s\n", flagBranch)
		fmt.Println()
	}

	// Prepare Config
	var analysisLoadedCount int64
	config := &RebuildConfig{
		GitPath:             gitPath,
		Owner:               flagOwner,
		Repo:                flagRepo,
		Branch:              flagBranch,
		DBConn:              dbConn,
		RefChatID:           refChatID,
		ShadowPath:          shadowPath,
		GSCHome:             gscHome,
		AnalysisLoadedCount: &analysisLoadedCount,
	}

	// Define Import Callback
	// This function performs the actual gscb-cli import (Stage 5)
	importFn := func() error {
		return executeImportAndSaveState(gitPath, gscHome, dbPath, shadowPath, flagOwner, flagRepo, flagBranch)
	}

	// Run Rebuild
	ctx := context.Background()
	if err := RunRebuild(ctx, config, importFn); err != nil {
		cmd.SilenceUsage = true
		return fmt.Errorf("rebuild failed: %w", err)
	}

	// Post-Rebuild Summary
	fmt.Println("\n✓ Rebuild complete")
	fmt.Println()
	fmt.Println("Summary:")
	fmt.Printf("  Repository: %s/%s (%s)\n", flagOwner, flagRepo, flagBranch)
	fmt.Printf("  Mode:       Rebuild (Fresh Import)\n")
	
	// Display analysis restoration status if applicable
	if analysisLoadedCount > 0 {
		fmt.Println()
		fmt.Println("Analysis Restoration:")
		fmt.Printf("  • Status: Restored\n")
		fmt.Printf("  • Records: %d loaded from backup\n", analysisLoadedCount)
	}
	
	fmt.Println()
	fmt.Println("⚠️  IMPORTANT: Chat IDs Have Changed")
	fmt.Println()
	fmt.Println("All files in this branch now have new chat IDs.")
	fmt.Println("If you have manifest files or brains that reference specific chat IDs,")
	fmt.Println("they will need to be recreated to use the new IDs.")
	fmt.Println()
	
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Println("  • Update later: gsc app import git --update")

	return nil
}

// executeImportAndSaveState encapsulates the logic to run gscb-cli and save state.
// This is used by both the standard import flow and the rebuild workflow.
func executeImportAndSaveState(gitPath, gscHome, dbPath, shadowPath, owner, repo, branch string) error {
	// Resolve GSCB CLI Path
	gscbCliPath := filepath.Join(gscHome, "node_modules", "@gitsense", "gscb-cli", "dist", "bin", "gscb.js")
	if _, err := os.Stat(gscbCliPath); os.IsNotExist(err) {
		return fmt.Errorf("gscb-cli not found at %s", gscbCliPath)
	}

	// Initialize ProgressUI
	progressUI := NewProgressUI()

	// Recreate shadow repository for rebuild
	// In the rebuild workflow, the shadow was deleted in Stage 3, so we must recreate it here.
	progressUI.StartShadowPhase()
	logger.Info("Recreating shadow repository...", "path", shadowPath)
	if err := CreateShadow(shadowPath, gitPath, branch, progressUI.ShadowProgressFn()); err != nil {
		return fmt.Errorf("failed to recreate shadow for rebuild: %w", err)
	}

	// Determine import path (always shadow for rebuild/new import)
	importPath := shadowPath

	// Build args
	cmdArgs := []string{
		gscbCliPath,
		"import", "git",
		importPath,
		owner,
		repo,
		branch,
		"--db-path", dbPath,
		"--display-repo-path", gitPath, // Always show source path
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
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	execCmd.Stderr = os.Stderr

	logger.Debug("Starting gscb-cli (rebuild)", "args", cmdArgs)

	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("failed to start gscb-cli: %w", err)
	}

	// Parse Stream
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
		return fmt.Errorf("error reading gscb-cli output: %w", err)
	}

	// Wait for completion
	if err := execCmd.Wait(); err != nil {
		return fmt.Errorf("gscb-cli process failed: %w", err)
	}

	// Get RefChatID
	refChatID := progressUI.GetRefChatID()
	if refChatID == 0 {
		return fmt.Errorf("import completed but no ref_chat_id was received")
	}

	// Get GroupID
	dbConn, err := db.OpenDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database to fetch group_id: %w", err)
	}
	defer db.CloseDB(dbConn)

	var groupID int64
	err = dbConn.QueryRow("SELECT group_id FROM chats WHERE id = ? AND deleted = 0", refChatID).Scan(&groupID)
	if err != nil {
		return fmt.Errorf("failed to query group_id for ref %d: %w", refChatID, err)
	}

	// Save State
	importFlags := ImportFlags{
		MaxSize:       flagMaxSize,
		Include:       flagInclude,
		Exclude:       flagExclude,
		IncludeBinary: flagIncludeBinary,
	}

	if err := SaveState(gitPath, owner, repo, branch, refChatID, groupID, importFlags, true, shadowPath); err != nil {
		logger.Warning("Failed to save import state", "error", err)
	}

	// Cleanup UI
	progressUI.Cleanup()

	// Print Summary
	stateFileRelPath := filepath.Join(".gitsense", stateFileName)
	progressUI.PrintShadowFinalSummary(owner, repo, branch, dbPath, filepath.Join(gitPath, stateFileRelPath), shadowPath, progressUI.GetDuration())

	return nil
}
