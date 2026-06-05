/*
 * Component: Analysis Dump Command
 * Block-UUID: 6bba6e09-e378-4ad6-949f-ea8a465e2e3c
 * Parent-UUID: e0c2d4a6-0979-4e2d-9504-6aed16e3e488
 * Version: 1.7.0
 * Description: Implements the 'gsc app analysis dump' command, handling flag parsing, context resolution, user confirmation, and orchestration of the database dump operation to JSONL. v1.6.0: Added incremental dump support using analysis_hash. Reads existing dump file to build hash set before dumping. Updated summary and result output to show written and skipped counts. v1.6.1: Fixed type mismatch error by casting alreadyDumped to int64. v1.6.2: Fixed bug where local state GroupID took precedence over explicit --owner and --repo flags. Now always queries database using resolved owner/repo to ensure flags are respected. v1.7.0: Updated db.DumpAnalysis call to pass owner and repo separately for portable dump files.
 * Language: Go
 * Created-at: 2026-05-16T12:48:33.996Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0), GLM-4.7 (v1.6.0), GLM-4.7 (v1.6.1), GLM-4.7 (v1.6.2), GLM-4.7 (v1.7.0)
 */


package analysis

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/cli/app/import/git"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	flagDumpAnalyzer string
	flagDumpOwner    string
	flagDumpRepo     string
	flagDumpBranch   string
	flagDumpForce    bool
)

// DumpCmd represents the dump analysis command
var DumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump analysis results to a JSONL file",
	Long: `Dumps AI-generated analysis metadata (e.g., code-intent) to a standalone JSONL file.
This allows you to backup analysis results outside of the database and restore them later.`,
	RunE: runDump,
}

func init() {
	// Required flags
	DumpCmd.Flags().StringVar(&flagDumpAnalyzer, "analyzer", "", "Analyzer type to dump (e.g., code-intent)")

	// Optional flags
	DumpCmd.Flags().StringVar(&flagDumpOwner, "owner", "", "Repository owner (inferred from state if omitted)")
	DumpCmd.Flags().StringVar(&flagDumpRepo, "repo", "", "Repository name (inferred from state if omitted)")
	DumpCmd.Flags().StringVar(&flagDumpBranch, "branch", "", "Branch name (inferred from git HEAD if omitted)")

	// Control flags
	DumpCmd.Flags().BoolVarP(&flagDumpForce, "force", "f", false, "Skip confirmation prompt")
}

// runDump executes the dump analysis workflow
func runDump(cmd *cobra.Command, args []string) error {
	// v1.5.0: Silence usage on error
	cmd.SilenceUsage = true

	// 1. Resolve Git Path (Optional, for context)
	gitPath, err := git.FindGitRoot()
	if err != nil {
		// Not a fatal error, we can proceed with flags
		gitPath = ""
	}

	// 2. Load Import State (Optional)
	var state *importgit.ImportState
	if gitPath != "" {
		state, err = importgit.LoadState(gitPath)
		if err != nil {
			// State file missing is OK, we'll use flags
			state = nil
		}
	}

	// 3. Resolve --owner
	owner := flagDumpOwner
	if owner == "" && state != nil {
		owner = state.Owner
	}
	if owner == "" {
		return fmt.Errorf("--owner is required (could not infer from state)")
	}

	// 4. Resolve --repo
	repo := flagDumpRepo
	if repo == "" && state != nil {
		repo = state.Repo
	}
	if repo == "" {
		return fmt.Errorf("--repo is required (could not infer from state)")
	}

	// 5. Resolve --branch
	branch := flagDumpBranch
	if branch == "" {
		if gitPath != "" {
			// Infer from current git branch
			cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			cmdBranch.Dir = gitPath
			output, err := cmdBranch.Output()
			if err == nil {
				branch = strings.TrimSpace(string(output))
			}
		}
	}
	if branch == "" {
		return fmt.Errorf("--branch is required (could not infer from git HEAD)")
	}

	// 6. Validate --analyzer
	if flagDumpAnalyzer == "" {
		return fmt.Errorf("--analyzer is required")
	}

	// 7. Open Database
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("failed to resolve GSC_HOME: %w", err)
	}

	dbPath := settings.GetChatDatabasePath(gscHome)
	if err := db.ValidateDBExists(dbPath); err != nil {
		return fmt.Errorf("database not found: %w", err)
	}

	dbConn, err := db.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// 8. Resolve GroupID
	// We use group_id to find the current refChatId for the branch, ensuring stability across re-imports
	// v1.6.2: Always query database using resolved owner/repo to ensure explicit flags take precedence
	groupID, err := db.GetGroupID(dbConn, owner, repo)
	if err != nil {
		return err
	}

	// 9. Resolve RefChatID
	refID, err := db.GetRefChatID(dbConn, groupID, branch)
	if err != nil {
		return err
	}

	// 10. Pre-flight Count
	count, err := db.CountAnalysisDump(dbConn, refID, flagDumpAnalyzer)
	if err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}

	if count == 0 {
		fmt.Println("No analysis records found to dump.")
		return nil
	}

	// 11. Construct Output Path
	// Extract analyzer root (first segment before ::)
	analyzerRoot := strings.Split(flagDumpAnalyzer, "::")[0]
	
	// Sanitize branch name (replace / with ::)
	sanitizedBranch := strings.ReplaceAll(branch, "/", "::")
	
	// Construct full path: ~/.gitsense/data/analysis/{analyzer}/{owner}/{repo}/{branch}.jsonl
	relPath := filepath.Join("data", "analysis", analyzerRoot, owner, repo, fmt.Sprintf("%s.jsonl", sanitizedBranch))
	absPath := filepath.Join(gscHome, relPath)

	// 11.5. Build Existing Hash Set (for incremental dumping)
	// Read the existing dump file to build a set of known analysis hashes
	existingHashes, err := db.BuildExistingHashSet(absPath)
	if err != nil {
		return fmt.Errorf("failed to read existing dump file: %w", err)
	}
	alreadyDumped := len(existingHashes)

	// Check if file exists to determine mode
	mode := "Create"
	if _, err := os.Stat(absPath); err == nil {
		mode = "Append"
	}

	// 12. Print Summary
	repoString := fmt.Sprintf("%s/%s", owner, repo)
	fmt.Println("\nReady to dump analysis:")
	fmt.Printf("  Analyzer:    %s\n", flagDumpAnalyzer)
	fmt.Printf("  Repository:  %s\n", repoString)
	fmt.Printf("  Branch:      %s\n", branch)
	fmt.Printf("  Records:     %d\n", count)
	fmt.Printf("  Destination: %s\n", absPath)
	fmt.Printf("  Mode:        %s\n", mode)
	fmt.Println()
	fmt.Println("  Incremental Dump:")
	fmt.Printf("    Already in dump:  %6d\n", alreadyDumped)
	fmt.Printf("    New records:      %6d  (estimated)\n", count-int64(alreadyDumped))
	fmt.Println()

	// 13. Confirmation
	if !flagDumpForce {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Proceed?",
			Default: true,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// 14. Ensure Directory Exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 15. Open File (Append Mode)
	file, err := os.OpenFile(absPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
	}
	defer file.Close()

	// 16. Start Spinner
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = fmt.Sprintf(" Dumping %d records...", count)
	s.Color("bold", "fgCyan")
	s.Start()

	// 17. Execute Dump
	dumpID := uuid.New().String()
	startTime := time.Now()
	
	// v1.7.0: Pass owner and repo separately to db.DumpAnalysis
	written, skipped, err := db.DumpAnalysis(dbConn, refID, flagDumpAnalyzer, dumpID, owner, repo, branch, file, existingHashes)

	// 18. Stop Spinner
	s.Stop()

	if err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}

	duration := time.Since(startTime)

	// 19. Print Result
	fmt.Printf("✓ Successfully dumped %d new records (%d skipped as duplicates) in %s.\n", written, skipped, duration.Round(time.Millisecond))
	fmt.Printf("  Dump ID: %s\n", dumpID)
	fmt.Printf("  File:    %s\n", absPath)

	return nil
}
