/**
 * Component: Import Command Root
 * Block-UUID: e7b078c8-3fe1-44cf-bdfe-cee8bed86039
 * Parent-UUID: 751cb208-8813-4ce8-9e78-6504636600e0
 * Version: 1.0.2
 * Description: Fixed typo in import path from csc-cli to gsc-cli.
 * Language: Go
 * Created-at: 2026-05-13T19:11:01.322Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), Gemini 3 Flash (v1.0.2)
 */


package importcmd

import (
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/cli/app/import/git"
)

// ImportCmd represents the base command for importing data
var ImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import data sources into the GitSense Chat database",
	Long: `The import command suite allows you to import various data sources 
into the GitSense Chat database for querying and context management.`,
}

// RegisterCommand adds the import command and its subcommands to a parent command
func RegisterCommand(parent *cobra.Command) {
	parent.AddCommand(ImportCmd)
}

func init() {
	// Register subcommands (e.g., git)
	importgit.RegisterCommand(ImportCmd)
}
