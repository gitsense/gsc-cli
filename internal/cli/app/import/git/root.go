/**
 * Component: Import Git Command Root
 * Block-UUID: 75d54519-7b86-41e8-afbb-30181eba1785
 * Parent-UUID: 2ff003d8-645f-4e86-8e10-36e60312a280
 * Version: 1.5.0
 * Description: Defines the 'git' subcommand for importing Git repositories, including all command-line flags. v1.5.0: Added --rebuild and --resume flags to support the full branch rebuild workflow.
 * Language: Go
 * Created-at: 2026-05-14T23:31:54.656Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package importgit

import (
	"github.com/spf13/cobra"
)

var (
	flagBranch        string
	flagRepo          string
	flagOwner         string
	flagPath          string
	flagDBPath        string
	flagMaxSize       int
	flagInclude       string
	flagExclude       string
	flagIncludeBinary bool
	flagVerbose       bool
	flagForce         bool
	flagUpdate        bool
	flagShadow        bool
	flagDeleteShadow  bool
	flagStatus        bool
	flagRebuild       bool
	flagResume        bool
)

// GitCmd represents the git import command
var GitCmd = &cobra.Command{
	Use:   "git",
	Short: "Import a Git repository into the GitSense Chat database",
	Long: `Imports a Git repository into the GitSense Chat database, enabling 
file-level context and history queries. Supports full imports and incremental 
updates via state files.`,
	RunE: runGit,
}

// RegisterCommand adds the git command to the parent import command
func RegisterCommand(parent *cobra.Command) {
	parent.AddCommand(GitCmd)
}

func init() {
	// Required flags (unless --update is used)
	GitCmd.Flags().StringVar(&flagBranch, "branch", "", "Branch or ref to import (required unless --update)")
	GitCmd.Flags().StringVar(&flagRepo, "repo", "", "Repository name (required unless --update)")
	GitCmd.Flags().StringVar(&flagOwner, "owner", "", "Repository owner/org (required unless --update)")

	// Optional flags
	GitCmd.Flags().StringVar(&flagPath, "path", "", "Path to git working directory (defaults to auto-detect)")
	GitCmd.Flags().StringVar(&flagDBPath, "db-path", "", "Explicit path to chats.sqlite3")
	GitCmd.Flags().IntVar(&flagMaxSize, "max-size", 0, "Max file size in KB (passed through to gscb-cli)")
	GitCmd.Flags().StringVar(&flagInclude, "include", "", "Regex include pattern (passed through)")
	GitCmd.Flags().StringVar(&flagExclude, "exclude", "", "Regex exclude pattern (passed through)")
	GitCmd.Flags().BoolVar(&flagIncludeBinary, "include-binary", false, "Include binary files (passed through)")
	GitCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Verbose logging (passed through)")

	// Control flags
	GitCmd.Flags().BoolVar(&flagForce, "force", false, "Skip confirmation prompts and enforce clean working tree for updates (fails fast if dirty)")
	GitCmd.Flags().BoolVar(&flagUpdate, "update", false, "Perform an incremental update using saved state")
	GitCmd.Flags().BoolVar(&flagStatus, "status", false, "Check shadow repository status without importing")

	// Phase 2: Shadow Repo flags
	// Note: Shadow mode is now enforced for all new imports
	GitCmd.Flags().BoolVar(&flagShadow, "shadow", false, "Force shadow mode (single-commit snapshot) [deprecated: now default]")
	GitCmd.Flags().BoolVar(&flagDeleteShadow, "delete-shadow", false, "Delete shadow repo(s)")

	// Rebuild flags
	GitCmd.Flags().BoolVar(&flagRebuild, "rebuild", false, "Force a full rebuild of the branch (dump analysis, delete shadow, delete DB, re-import, restore analysis)")
	GitCmd.Flags().BoolVar(&flagResume, "resume", false, "Resume a failed rebuild operation from the last checkpoint")
}
