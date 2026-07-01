/**
 * Component: Analysis Delete Command
 * Block-UUID: 7a8b9c0d-1e2f-3a4b-5c6d-7e8f9a0b1c2d
 * Parent-UUID: df0719b7-c5bd-43c5-95bc-36e4e465a4b3
 * Version: 1.0.0
 * Description: Implements the 'gsc app analysis delete' command for removing all analysis metadata for a specific analyzer across all files in a branch. Performs soft-delete with message chain re-linking and chat metadata cleanup. Supports dry-run mode for safe preview.
 * Language: Go
 * Created-at: 2026-06-29T00:00:00.000Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */


package analysis

import (
	"fmt"
	"os"

	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/spf13/cobra"
)

var (
	flagDeleteAnalyzer  string
	flagDeleteRefChatID int64
	flagDeleteDryRun    bool
	flagDeleteForce     bool
	flagDeleteOwner     string
	flagDeleteRepo      string
	flagDeleteBranch    string
)

// DeleteCmd represents the 'gsc app analysis delete' command.
var DeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Remove all analysis data for an analyzer",
	Long: `Removes all analysis messages for a specific analyzer across all files in a branch.

This is useful when analyzer data gets corrupted and needs to be rebuilt from scratch.
The command performs a soft-delete (sets deleted=1) and preserves the message chain
integrity by re-linking child messages before deletion.

Operations performed:
  1. Re-links child messages to maintain the conversation chain
  2. Archives deleted messages to message_history for auditability
  3. Removes the analyzer entry from each file's chat metadata (tokens.analysis)

Examples:
  # Preview what would be deleted (dry run)
  gsc app analysis delete --analyzer rust-depmap --dry-run

  # Delete all analysis for an analyzer
  gsc app analysis delete --analyzer rust-depmap

  # Delete with explicit branch context
  gsc app analysis delete --analyzer code-intent --owner myorg --repo myrepo --branch main

  # Force delete without confirmation prompt
  gsc app analysis delete --analyzer rust-depmap --force`,
	RunE: runDelete,
}

func init() {
	DeleteCmd.Flags().StringVar(&flagDeleteAnalyzer, "analyzer", "", "Analyzer name (e.g., rust-depmap). Required.")
	_ = DeleteCmd.MarkFlagRequired("analyzer")

	DeleteCmd.Flags().Int64Var(&flagDeleteRefChatID, "ref-chat-id", 0, "Branch ref chat ID for disambiguation")
	DeleteCmd.Flags().BoolVar(&flagDeleteDryRun, "dry-run", false, "Preview what would be deleted without making changes")
	DeleteCmd.Flags().BoolVar(&flagDeleteForce, "force", false, "Skip confirmation prompt")
	DeleteCmd.Flags().StringVar(&flagDeleteOwner, "owner", "", "Repository owner (overrides auto-detection)")
	DeleteCmd.Flags().StringVar(&flagDeleteRepo, "repo", "", "Repository name (overrides auto-detection)")
	DeleteCmd.Flags().StringVar(&flagDeleteBranch, "branch", "", "Branch name (overrides auto-detection)")
}

// runDelete is the main entry point for the delete command.
func runDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// 1. Open database
	dbConn, err := openChatDB()
	if err != nil {
		return err
	}
	defer db.CloseDB(dbConn)

	// 2. Resolve branch context
	refChatID, err := resolveRefChatID(dbConn, flagDeleteRefChatID, flagDeleteOwner, flagDeleteRepo, flagDeleteBranch)
	if err != nil {
		return err
	}

	// 3. Run dry-run first to get counts
	result, err := db.DeleteAnalysisForAnalyzer(dbConn, refChatID, flagDeleteAnalyzer, true)
	if err != nil {
		return fmt.Errorf("failed to analyze delete scope: %w", err)
	}

	// 4. Print summary
	fmt.Fprintf(os.Stderr, "\nDelete Analysis Summary:\n")
	fmt.Fprintf(os.Stderr, "  Analyzer:        %s\n", flagDeleteAnalyzer)
	fmt.Fprintf(os.Stderr, "  Messages:        %d to delete\n", result.DeletedMessages)
	fmt.Fprintf(os.Stderr, "  Files affected:  %d\n", result.UpdatedChats)
	fmt.Fprintf(os.Stderr, "\n")

	if result.DeletedMessages == 0 {
		fmt.Fprintln(os.Stderr, "No analysis messages found for this analyzer.")
		return nil
	}

	// 5. Dry run: exit after preview
	if flagDeleteDryRun {
		fmt.Fprintln(os.Stderr, "Dry run complete. No changes made.")
		return nil
	}

	// 6. Confirmation prompt (unless --force)
	if !flagDeleteForce {
		fmt.Fprintf(os.Stderr, "This will soft-delete %d analysis messages and remove metadata from %d files.\n",
			result.DeletedMessages, result.UpdatedChats)
		fmt.Fprintf(os.Stderr, "Messages will be archived to message_history before deletion.\n\n")
		fmt.Fprintf(os.Stderr, "Continue? [y/N]: ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	// 7. Execute actual delete
	result, err = db.DeleteAnalysisForAnalyzer(dbConn, refChatID, flagDeleteAnalyzer, false)
	if err != nil {
		return fmt.Errorf("delete operation failed: %w", err)
	}

	// 8. Print result summary (JSON to stdout)
	summary := map[string]interface{}{
		"analyzer":        flagDeleteAnalyzer,
		"deleted_messages": result.DeletedMessages,
		"updated_chats":   result.UpdatedChats,
		"reparented_links": result.ReparentedLinks,
	}

	if err := outputJSON(summary); err != nil {
		return fmt.Errorf("failed to encode summary: %w", err)
	}

	return nil
}
