/**
 * Component: Native App CLI Root
 * Block-UUID: 44ce886f-d8be-4b9c-bf01-e8fed396a84f
 * Parent-UUID: 9a9a3d6b-833f-4d51-bf46-632134177c7a
 * Version: 1.3.0
 * Description: Defines the root 'native' command group for managing the native GitSense Chat application lifecycle, now supporting start, stop, and restart.
 * Language: Go
 * Created-at: 2026-05-12T15:44:09.797Z
 * Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
 */


package native

import (
	"github.com/spf13/cobra"
)

// NativeCmd represents the base command for native application management
var NativeCmd = &cobra.Command{
	Use:   "native",
	Short: "Manage the native GitSense Chat application lifecycle",
	Long: `The native command suite allows you to manage the lifecycle of the 
native GitSense Chat application, including starting, stopping, 
restarting the service, and managing configuration.`,
}

// RegisterCommand adds the native command and its subcommands to a parent command
func RegisterCommand(parent *cobra.Command) {
	parent.AddCommand(NativeCmd)
}

func init() {
	// Subcommands (start, stop, restart, admin) are registered by their respective files.
}
