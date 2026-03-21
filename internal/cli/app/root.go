/*
 * Component: App CLI Root
 * Block-UUID: ec2931f0-5a37-4524-9915-159279e5a7e9
 * Parent-UUID: 4a8e8a03-be4b-423d-a16d-c81b46780949
 * Version: 1.1.0
 * Description: Defines the root 'app' command group for managing the native GitSense Chat application lifecycle, now supporting start, stop, and restart.
 * Language: Go
 * Created-at: 2026-03-20T23:00:44.410Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0)
 */


package app

import (
	"github.com/spf13/cobra"
)

// AppCmd represents the base command for native application management
var AppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage the native GitSense Chat application lifecycle",
	Long: `The app command suite allows you to manage the lifecycle of the 
native GitSense Chat application, including starting, stopping, and 
restarting the service.`,
}

// RegisterCommand adds the app command and its subcommands to the root CLI
func RegisterCommand(root *cobra.Command) {
	root.AddCommand(AppCmd)
}

func init() {
	// Subcommands (start, stop, restart) are registered by their respective files.
}
