/**
 * Component: Docs Admin Command
 * Block-UUID: 8a9b2c3d-4e5f-6a7b-8c9d-0e1f2a3b4c5d
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs admin' that displays the configuration management guide for LLM providers, models, and environment variables.
 * Language: Go
 * Created-at: 2026-05-31T08:25:00.000Z
 * Authors: DeepSeek V4 Pro (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewAdminCmd creates and returns the 'gsc docs admin' subcommand.
func NewAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "How to configure LLM providers, models, and environment variables",
		Long: `Displays the configuration management guide that covers managing LLM
providers, models, and environment variables (.env file) for the GitSense
Chat web application.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("admin")
		},
	}

	return cmd
}
