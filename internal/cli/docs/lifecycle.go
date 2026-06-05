/**
 * Component: Docs Lifecycle Command
 * Block-UUID: cee93ef4-5710-469f-938d-643f1b0dc899
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs lifecycle' that displays the application lifecycle management guide covering start, stop, restart, status, and logs.
 * Language: Go
 * Created-at: 2026-05-31T08:20:00.000Z
 * Authors: DeepSeek V4 Pro (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewLifecycleCmd creates and returns the 'gsc docs lifecycle' subcommand.
func NewLifecycleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lifecycle",
		Short: "How to start, stop, restart, and monitor the GitSense Chat app",
		Long: `Displays the lifecycle management guide that covers starting, stopping,
restarting, checking status, and viewing logs for both Native and Docker
deployments of the GitSense Chat web application.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("lifecycle")
		},
	}

	return cmd
}
