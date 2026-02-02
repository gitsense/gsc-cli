/*
 * Component: Manifest Bundle Command
 * Block-UUID: 612e37d9-20da-4ccb-8ad1-6ffc95312246
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command definition for generating context bundles from a manifest database using SQL queries.
 * Language: Go
 * Created-at: 2026-02-02T08:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var (
	bundleQuery string
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
			logger.Error(fmt.Sprintf("Bundle generation failed: %v", err))
			return err
		}

		fmt.Println(output)
		return nil
	},
}

func init() {
	// Add flags
	bundleCmd.Flags().StringVarP(&bundleQuery, "query", "q", "", "SQL query to execute (e.g., 'SELECT file_path, chat_id FROM files WHERE language=\"javascript\"')")
	bundleCmd.Flags().StringVarP(&bundleFormat, "format", "f", "context-list", "Output format (context-list, json)")
	
	// Mark query as required
	bundleCmd.MarkFlagRequired("query")
}

// GetBundleCommand returns the bundle command for registration
func GetBundleCommand() *cobra.Command {
	return bundleCmd
}
