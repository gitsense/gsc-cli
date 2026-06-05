/**
 * Component: Analysis Load Command
 * Block-UUID: c4c7d253-1dc2-4348-b94e-5336c3b7b472
 * Parent-UUID: 6ab2b3aa-df7e-4938-b9d0-3fab4fe32c8c
 * Version: 1.1.1
 * Description: Implements the 'gsc app analysis load' command, handling flag parsing, context resolution, JSONL ingestion, preflight analysis, and orchestration of the database load operation. Supports graceful interruption and batched transactions. v1.1.0: Replaced --analyzer flag with --dump-file. Added support for portable dump files with embedded metadata. Implemented three-tier inference for target resolution (flags -> state -> dump header). Added safety validation for source/target mismatches.
 * Language: Go
 * Created-at: 2026-05-16T18:40:30.003Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3), GLM-4.7 (v1.0.4), GLM-4.7 (v1.1.0), GLM-4.7 (v1.1.1)
 */


package analysis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/cli/app/import/git"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/gitsense/gsc-cli/internal/git"
	"github.com/gitsense/gsc-cli/pkg/settings"
)

var (
	flagLoadDumpFile string
	flagLoadOwner    string
	flagLoadRepo     string
	flagLoadBranch   string
	flagLoadForce    bool
)

// LoadCmd represents the load analysis command
var LoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load analysis results from a JSONL file",
	Long: `Loads AI-generated analysis metadata (e.g., code-intent) from a JSONL file
into the database. This is useful for restoring analysis results after a database
wipe or re-import. Supports portable dump files that contain embedded metadata.`,
	RunE: runLoad,
}

func init() {
	// Required flags
	LoadCmd.Flags().StringVar(&flagLoadDumpFile, "dump-file", "", "Path to the JSONL dump file to load (required)")
	LoadCmd.MarkFlagRequired("dump-file")

	// Optional flags
	LoadCmd.Flags().StringVar(&flagLoadOwner, "owner", "", "Repository owner (inferred from state or dump file if omitted)")
	LoadCmd.Flags().StringVar(&flagLoadRepo, "repo", "", "Repository name (inferred from state or dump file if omitted)")
	LoadCmd.Flags().StringVar(&flagLoadBranch, "branch", "", "Branch name (inferred from git HEAD or dump file if omitted)")

	// Control flags
	LoadCmd.Flags().BoolVarP(&flagLoadForce, "force", "f", false, "Skip confirmation prompt and safety checks")
}

// runLoad executes the load analysis workflow
func runLoad(cmd *cobra.Command, args []string) error {
	// v1.0.3: Silence usage on error
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

	// 3. Validate --dump-file exists
	if _, err := os.Stat(flagLoadDumpFile); os.IsNotExist(err) {
		return fmt.Errorf("dump file not found: %s", flagLoadDumpFile)
	}
	absPath := flagLoadDumpFile

	// 4. Read First Line for Source Metadata (Portable Dump File Support)
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open dump file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// v1.0.3: Increase scanner buffer to 1MB to handle large JSONL records
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	// Read header line
	if !scanner.Scan() {
		return fmt.Errorf("dump file is empty: %s", absPath)
	}

	var headerRecord db.DumpRecord
	if err := json.Unmarshal(scanner.Bytes(), &headerRecord); err != nil {
		return fmt.Errorf("failed to parse dump file header: %w", err)
	}

	// Backward Compatibility: Handle old dump files where Owner is empty and Repo contains "owner/repo"
	if headerRecord.Owner == "" && strings.Contains(headerRecord.Repo, "/") {
		parts := strings.SplitN(headerRecord.Repo, "/", 2)
		headerRecord.Owner = parts[0]
		headerRecord.Repo = parts[1]
	}

	// 5. Resolve Target Owner (Three-tier inference: Flags -> State -> Dump Header)
	owner := flagLoadOwner
	if owner == "" && state != nil {
		owner = state.Owner
	}
	if owner == "" {
		owner = headerRecord.Owner
	}
	if owner == "" {
		return fmt.Errorf("--owner is required (could not infer from state or dump file)")
	}

	// 6. Resolve Target Repo (Three-tier inference: Flags -> State -> Dump Header)
	repo := flagLoadRepo
	if repo == "" && state != nil {
		repo = state.Repo
	}
	if repo == "" {
		repo = headerRecord.Repo
	}
	if repo == "" {
		return fmt.Errorf("--repo is required (could not infer from state or dump file)")
	}

	// 7. Resolve Target Branch (Three-tier inference: Flags -> Git HEAD -> Dump Header)
	branch := flagLoadBranch
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
		branch = headerRecord.Branch
	}
	if branch == "" {
		return fmt.Errorf("--branch is required (could not infer from git HEAD or dump file)")
	}

	// 8. Safety Validation (Mismatch Checks)
	ownerRepoDiffers := (headerRecord.Owner != "" && headerRecord.Repo != "" && 
		(owner != headerRecord.Owner || repo != headerRecord.Repo))
	branchDiffers := (headerRecord.Branch != "" && branch != headerRecord.Branch)

	if ownerRepoDiffers && !flagLoadForce {
		return fmt.Errorf(
			"❌ Mismatch: dump file is for '%s/%s' but target is '%s/%s'. Use --force to override.",
			headerRecord.Owner, headerRecord.Repo, owner, repo,
		)
	}

	if branchDiffers {
		fmt.Printf("⚠️  Loading analysis from branch '%s' into branch '%s'.\n", headerRecord.Branch, branch)
	}

	// 9. Derive Analyzer Prefix from Dump File
	analyzerPrefix := headerRecord.Analyzer
	if analyzerPrefix == "" && headerRecord.MessageType != "" {
		// Fallback to message type if analyzer field is missing
		analyzerPrefix = strings.Split(headerRecord.MessageType, "::")[0]
	}
	if analyzerPrefix == "" {
		return fmt.Errorf("could not determine analyzer type from dump file")
	}
	// Convert to prefix for DB lookup if not already a full type
	if !strings.Contains(analyzerPrefix, "::") {
		analyzerPrefix = analyzerPrefix + "::%"
	}

	// 10. Open Database
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

	// 11. Resolve GroupID
	groupID, err := db.GetGroupID(dbConn, owner, repo)
	if err != nil {
		return err
	}

	// 12. Resolve RefChatID
	refID, err := db.GetRefChatID(dbConn, groupID, branch)
	if err != nil {
		return err
	}

	// 13. Build Target Snapshot (DB -> RAM)
	targetMap, err := db.BuildTargetSnapshot(dbConn, refID, analyzerPrefix)
	if err != nil {
		return fmt.Errorf("failed to build target snapshot: %w", err)
	}

	// 14. Ingest JSONL (File -> RAM)
	// Note: We already read the first line (headerRecord), so we continue scanning from here
	sourceMap := make(map[string]db.DumpRecord)
	
	// Add the header record to the map (it's valid data)
	sourceMap[headerRecord.Path] = headerRecord
	
	lineCount := 1 // We already read line 1

	for scanner.Scan() {
		lineCount++
		var record db.DumpRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return fmt.Errorf("failed to parse JSONL line %d: %w", lineCount, err)
		}
		// Deduplicate: later entries overwrite earlier ones
		sourceMap[record.Path] = record
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading dump file: %w", err)
	}

	// 15. Preflight Analysis (Intersection)
	totalBranchFiles := len(targetMap)
	var candidates []db.LoadItem
	ignoredCount := 0
	alreadyAnalyzedCount := 0

	for path, record := range sourceMap {
		if state, ok := targetMap[path]; ok {
			if !state.HasAnalysis {
				candidates = append(candidates, db.LoadItem{
					Path:     path,
					ChatID:   state.ChatID,
					ParentID: state.ParentID,
					Record:   record,
				})
			} else {
				alreadyAnalyzedCount++
			}
		} else {
			ignoredCount++
		}
	}

	notInDumpCount := totalBranchFiles - len(candidates) - alreadyAnalyzedCount

	// 16. Print Summary
	fmt.Println("\nReady to load analysis:")
	fmt.Printf("  Source file: %s\n", absPath)
	fmt.Printf("  Source:      %s/%s (%s)\n", headerRecord.Owner, headerRecord.Repo, headerRecord.Branch)
	fmt.Printf("  Target:      %s/%s (%s)\n", owner, repo, branch)
	fmt.Printf("  Records:     %d in dump file\n", len(sourceMap))
	fmt.Println()
	fmt.Println("  Analysis:")
	fmt.Printf("    Total files in branch:  %6d\n", totalBranchFiles)
	fmt.Printf("    Files to update:        %6d  (%3d%% coverage)\n", len(candidates), calculatePercentage(len(candidates), totalBranchFiles))
	fmt.Printf("    Already analyzed:       %6d  (skipped)\n", alreadyAnalyzedCount)
	fmt.Printf("    Files not in dump:      %6d  (will not be updated)\n", notInDumpCount)
	fmt.Printf("    Records not in branch:  %6d  (ignored)\n", ignoredCount)
	fmt.Println()

	if len(candidates) == 0 {
		fmt.Println("No files to update.")
		return nil
	}

	// 17. Confirmation
	if !flagLoadForce {
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

	// 18. Setup Signal Handling
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Clean up signal handler

	go func() {
		<-sigChan
		fmt.Println("\n\nInterrupt received. Finishing current batch...")
		cancel()
	}()

	// 19. Progress Display
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Color("bold", "fgCyan")
	s.Suffix = " Loading analysis... 0/0 (0%)"
	s.Start()

	isFirstUpdate := true
	totalCandidates := len(candidates)

	// 20. Execute Load
	startTime := time.Now()
	
	loadedCount, err := db.LoadAnalysis(ctx, dbConn, candidates, 1000, func(n int, path string) {
		// Update Progress
		percentage := float64(n) / float64(totalCandidates) * 100
		s.Suffix = fmt.Sprintf(" Loading analysis... %d/%d (%.0f%%)", n, totalCandidates, percentage)

		if isFirstUpdate {
			fmt.Printf("   Current: %s\n", path)
			isFirstUpdate = false
		} else {
			// Clear previous 2 lines
			fmt.Print("\033[F\033[2K") // Clear file line
			fmt.Print("\033[F\033[2K") // Clear spinner line
			
			// Reprint spinner
			s.Restart()
			
			// Print new file line
			fmt.Printf("   Current: %s\n", path)
		}
	})

	// 21. Stop Spinner & Cleanup
	s.Stop()
	
	// Clear the lingering "Current: ..." line
	fmt.Print("\033[F\033[2K")

	if err != nil {
		return fmt.Errorf("load failed: %w", err)
	}

	duration := time.Since(startTime)

	// 22. Print Result
	fmt.Printf("✓ Successfully loaded %d analysis records in %s.\n", loadedCount, duration.Round(time.Millisecond))

	return nil
}

// calculatePercentage safely calculates a percentage
func calculatePercentage(part, total int) int {
	if total == 0 {
		return 0
	}
	return int(float64(part) / float64(total) * 100)
}
