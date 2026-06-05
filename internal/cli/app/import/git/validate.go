/*
 * Component: Import Git Validation
 * Block-UUID: 96922a2e-c374-4b5a-aa97-090359bd042d
 * Parent-UUID: e6be829f-3b36-4f82-acc2-3adbae9cab14
 * Version: 1.0.0
 * Description: Extracted validation logic from command.go to separate pre-flight checks from orchestration.
 * Language: Go
 * Created-at: 2026-05-17T15:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package importgit

import (
	"database/sql"
	"fmt"
	"os/exec"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gitsense/gsc-cli/internal/db"
	"github.com/spf13/cobra"
)

// checkBranchExists verifies if a branch exists in the repository
func checkBranchExists(repoRoot, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoRoot
	err := cmd.Run()
	return err == nil
}

// checkExistingChatImport validates whether an import already exists in the GitSense Chat Application
// database for the given repository and branch. If found, it warns the user that this will update
// the existing import.
func checkExistingChatImport(cmd *cobra.Command, dbConn *sql.DB, gitPath, owner, repo, branch, dbPath string) error {
	// Check if repository exists in database
	groupID, err := db.GetGroupID(dbConn, owner, repo)
	if err != nil {
		// Repository doesn't exist in database, no conflict
		return nil
	}

	// Check if branch exists in database
	_, err = db.GetRefChatID(dbConn, groupID, branch)
	if err != nil {
		// Branch doesn't exist in database, no conflict
		return nil
	}

	// Repository and branch exist - show warning to user
	fmt.Println("\n⚠️  Warning: An existing import was found in the GitSense Chat Application.")
	fmt.Println()
	fmt.Printf("  Repository: %s/%s\n", owner, repo)
	fmt.Printf("  Branch:     %s\n", branch)
	fmt.Printf("  Database:   %s\n", dbPath)
	fmt.Println()
	fmt.Println("This will update the existing import with new data from the repository.")
	fmt.Println()
	fmt.Println("Note: This check is for the GitSense Chat Application database, not the shadow repository.")
	fmt.Println("      The shadow repository is a local snapshot used for performance optimization.")
	fmt.Println()

	// Present options to user
	choice := ""
	prompt := &survey.Select{
		Message: "How would you like to proceed?",
		Options: []string{
			"Continue (update existing import)",
			"Cancel",
			"Delete existing import from Chat Application and start fresh",
		},
		Default: "Continue (update existing import)",
	}
	if err := survey.AskOne(prompt, &choice); err != nil {
		return err
	}

	switch choice {
	case "Continue (update existing import)":
		// User chose to continue, let the import proceed
		return nil
	case "Cancel":
		fmt.Println("Import cancelled.")
		cmd.SilenceUsage = true
		return fmt.Errorf("import cancelled by user")
	case "Delete existing import from Chat Application and start fresh":
		// Provide guidance on how to delete the import
		fmt.Println()
		fmt.Println("To delete the existing import from the GitSense Chat Application:")
		fmt.Println("  1. Open the GitSense Chat Application")
		fmt.Println("  2. Navigate to the repository/branch in the chat")
		fmt.Println("  3. Delete the repository or branch from the application")
		fmt.Println()
		fmt.Println("After deleting, you can run this import command again.")
		cmd.SilenceUsage = true
		return fmt.Errorf("import cancelled: please delete existing import from Chat Application first")
	default:
		return fmt.Errorf("invalid choice")
	}
}
