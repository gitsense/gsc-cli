/**
 * Component: Analysis Copy Command
 * Block-UUID: 8e9c2e7b-5f0b-4220-b2a6-0c533cb846af
 * Parent-UUID: 5e11eab0-8115-4dd1-bc88-a571e7c25200
 * Version: 2.3.0
 * Description: Implements the 'gsc app analysis copy' command using a dump-then-load approach. Dumps analysis from source branch to a JSONL file, then loads it into the target branch. The dump file is saved to the default location for backup and reuse. v2.0.0: Complete rewrite to use dump-then-load instead of direct SQL operations. Removed --dry-run flag. v2.0.1: Fixed flag setting to use Flags().Set() instead of incorrect Args() usage. v2.0.2: Added cmd.SilenceUsage = true to suppress usage display on errors. Improved error message for missing source branch to clarify it must be in import state. v2.1.0: Fixed branch validation to use database queries instead of import state file. Now validates branches using db.GetGroupID and db.GetRefChatID, consistent with dump and load commands. v2.2.0: Renamed --from and --to flags to --from-branch and --to-branch for better clarity and user orientation. Updated summary output to match.
 * Language: Go
 * Created-at: 2026-05-17T14:24:35.051Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.1.0), GLM-4.7 (v2.0.0), GLM-4.7 (v2.0.1), GLM-4.7 (v2.0.2), GLM-4.7 (v2.1.0), Gemini 3 Flash (v2.2.0), Gemini 2.5 Flash Lite (v2.3.0)
 */


package analysis

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/cli/app/import/git"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	flagCopyAnalyzer   string
	flagCopyFromBranch string
	flagCopyToBranch   string
	flagCopyOwner      string
	flagCopyRepo       string
	flagCopyForce      bool
)

// CopyCmd represents the copy analysis command
var CopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy analysis results from one branch to another",
	Long: `Copies analysis from one branch to another by dumping to a JSONL file
and then loading it into the target branch. The dump file is saved to the
default location and can be reused for other purposes.`,
	RunE: runCopy,
}

func init() {
	// Required flags
	CopyCmd.Flags().StringVar(&flagCopyAnalyzer, "analyzer", "", "Analyzer type to copy (e.g., code-intent)")

	// Optional flags
	CopyCmd.Flags().StringVar(&flagCopyFromBranch, "from-branch", "", "Source branch name (inferred from state if omitted)")
	CopyCmd.Flags().StringVar(&flagCopyToBranch, "to-branch", "", "Target branch name (inferred from git HEAD if omitted)")
	CopyCmd.Flags().StringVar(&flagCopyOwner, "owner", "", "Repository owner (inferred from state if omitted)")
	CopyCmd.Flags().StringVar(&flagCopyRepo, "repo", "", "Repository name (inferred from state if omitted)")

	// Control flags
	CopyCmd.Flags().BoolVarP(&flagCopyForce, "force", "f", false, "Skip confirmation prompt")
}

// runCopy executes the copy analysis workflow using dump-then-load
func runCopy(cmd *cobra.Command, args []string) error {
	// v2.0.2: Silence usage on error
	cmd.SilenceUsage = true

	// 1. Resolve Git Path (Optional, for context)
	gitPath, err := git.FindGitRoot()
	if err != nil {
		// Not a fatal error, we can proceed with flags
		gitPath = ""
	}

	// 2. Load Import State (Optional, for inference)
	var state *importgit.ImportState
	if gitPath != "" {
		state, err = importgit.LoadState(gitPath)
		if err != nil {
			// State file missing is OK, we'll use flags
			state = nil
		}
	}

	// 3. Resolve --owner
	owner := flagCopyOwner
	if owner == "" && state != nil {
		owner = state.Owner
	}
	if owner == "" {
		return fmt.Errorf("--owner is required (could not infer from state)")
	}

	// 4. Resolve --repo
	repo := flagCopyRepo
	if repo == "" && state != nil {
		repo = state.Repo
	}
	if repo == "" {
		return fmt.Errorf("--repo is required (could not infer from state)")
	}

	// 5. Resolve --to-branch (Target Branch)
	toBranch := flagCopyToBranch
	if toBranch == "" {
		if gitPath != "" {
			// Infer from current git branch
			cmdBranch := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
			cmdBranch.Dir = gitPath
			output, err := cmdBranch.Output()
			if err == nil {
				toBranch = strings.TrimSpace(string(output))
			}
		}
	}
	if toBranch == "" {
		return fmt.Errorf("--to-branch is required (could not infer from git HEAD)")
	}

	// 6. Resolve --from-branch (Source Branch)
	fromBranch := flagCopyFromBranch
	if fromBranch == "" {
		// Infer from state if available
		if state != nil {
			// If only one branch exists and it's not the target, use it
			if len(state.Branches) == 1 {
				for name := range state.Branches {
					if name != toBranch {
						fromBranch = name
						break
					}
				}
			} else if len(state.Branches) > 1 {
				// Multiple branches, require explicit flag
				return fmt.Errorf("multiple branches found in state, please specify --from-branch")
			}
		}
	}

	if fromBranch == "" {
		return fmt.Errorf("could not determine source branch, please specify --from-branch")
	}

	// 7. Validate --analyzer
	if flagCopyAnalyzer == "" {
		return fmt.Errorf("--analyzer is required")
	}

	// 8. Open Database
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

	// 9. Validate Repository Exists in Database
	groupID, err := db.GetGroupID(dbConn, owner, repo)
	if err != nil {
		return fmt.Errorf("repository '%s/%s' not found in database: %w", owner, repo, err)
	}

	// 10. Validate Source Branch Exists in Database
	_, err = db.GetRefChatID(dbConn, groupID, fromBranch)
	if err != nil {
		return fmt.Errorf("source branch '%s' not found in database for repository '%s/%s': %w", fromBranch, owner, repo, err)
	}

	// 11. Validate Target Branch Exists in Database
	_, err = db.GetRefChatID(dbConn, groupID, toBranch)
	if err != nil {
		return fmt.Errorf("target branch '%s' not found in database for repository '%s/%s': %w", toBranch, owner, repo, err)
	}

	// 12. Construct Dump File Path (same as dump.go default location)
	// Extract analyzer root (first segment before ::)
	analyzerRoot := strings.Split(flagCopyAnalyzer, "::")[0]
	
	// Sanitize branch name (replace / with ::)
	sanitizedBranch := strings.ReplaceAll(fromBranch, "/", "::")
	
	// Construct full path: ~/.gitsense/data/analysis/{analyzer}/{owner}/{repo}/{branch}.jsonl
	dumpPath := filepath.Join(gscHome, "data", "analysis", analyzerRoot, owner, repo, fmt.Sprintf("%s.jsonl", sanitizedBranch))

	// 13. Print Summary
	fmt.Println("\nReady to copy analysis:")
	fmt.Printf("  Source Branch: %s\n", fromBranch)
	fmt.Printf("  Target Branch: %s\n", toBranch)
	fmt.Printf("  Analyzer:      %s\n", flagCopyAnalyzer)
	fmt.Printf("  Dump file:     %s\n", dumpPath)
	fmt.Println()
	fmt.Println("This will dump analysis from the source branch to a JSONL file,")
	fmt.Println("then load it into the target branch. The dump file is saved for backup and reuse.")
	fmt.Println()

	// 14. Confirmation
	if !flagCopyForce {
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

	// 15. Execute Dump Phase
	fmt.Println("\n[1/2] Dumping analysis from source branch...")
	flagDumpAnalyzer = flagCopyAnalyzer
	flagDumpOwner = owner
	flagDumpRepo = repo
	flagDumpBranch = fromBranch
	flagDumpForce = true
	if err := runDump(DumpCmd, nil); err != nil {
		return fmt.Errorf("dump phase failed: %w", err)
	}

	// 16. Execute Load Phase
	fmt.Println("\n[2/2] Loading analysis into target branch...")
	flagLoadDumpFile = dumpPath
	flagLoadOwner = owner
	flagLoadRepo = repo
	flagLoadBranch = toBranch
	flagLoadForce = true
	if err := runLoad(LoadCmd, nil); err != nil {
		return fmt.Errorf("load phase failed: %w", err)
	}

	// 17. Print Result
	fmt.Println("\n✓ Successfully copied analysis.")
	fmt.Printf("  Source Branch: %s\n", fromBranch)
	fmt.Printf("  Target Branch: %s\n", toBranch)
	fmt.Printf("  Dump file:     %s\n", dumpPath)

	return nil
}

