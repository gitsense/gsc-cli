/*
 * Component: Manifest Bundle Command
 * Block-UUID: f2767aa4-33e1-4446-80f5-14ed2d5162ee
 * Parent-UUID: 535bc371-99a6-4b3b-8d58-fcb8cbb9f210
 * Version: 1.2.0
 * Description: CLI command definition for generating context bundles from a manifest database using SQL queries. Removed unused getter function. Updated to support professional CLI output: removed redundant logger.Error calls in RunE and set SilenceUsage to true to prevent usage spam on logic errors.
 * Language: Go
 * Created-at: 2026-02-02T08:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0), Claude Haiku 4.5 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/gitsense/gsc-cli/pkg/logger"
)

var (
	bundleQuery  string
	bundleFormat string
)

// bundleCmd represents the bundle command
var bundleCmd = &cobra.Command{
	Use:   "bundle <database-name>",
	Short: "Generate a context bundle from a manifest database",
	Long: `Generate a context bundle by executing a SQL query against the manifest database.
This is useful for creating focused lists of files for AI agents to analyze.
The 'context-list' format outputs lines like 'filename.ext (chat-id: 123)' which is
optimized for GitSense Chat context loading.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		logger.Info(fmt.Sprintf("Generating bundle from database '%s'...", dbName))

		ctx := context.Background()
		output, err := manifest.CreateBundle(ctx, dbName, bundleQuery, bundleFormat)
		if err != nil {
			// Error is returned to Cobra, which will print it cleanly via root.HandleExit
			return err
		}

		fmt.Println(output)
		return nil
	},
	SilenceUsage: true, // Silence usage output on logic errors (e.g., DB not found)
}

func init() {
	// Add flags
	bundleCmd.Flags().StringVarP(&bundleQuery, "query", "q", "", "SQL query to execute (e.g., 'SELECT file_path, chat_id FROM files WHERE language=\"javascript\"')")
	bundleCmd.Flags().StringVarP(&bundleFormat, "format", "f", "context-list", "Output format (context-list, json)")
	
	// Mark query as required
	bundleCmd.MarkFlagRequired("query")
}
