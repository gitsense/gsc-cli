/**
 * Component: Native App CLI Admin
 * Block-UUID: aaf7c292-6711-4dad-9a11-ee6abbac2bb9
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the root 'admin' command group for managing GitSense Chat configuration, providing a convenient wrapper around gsc-admin tools.
 * Language: Go
 * Created-at: 2026-05-12T15:40:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package native

import (
	"github.com/spf13/cobra"
)

// adminCmd represents the base command for admin operations
var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Manage GitSense Chat configuration",
	Long: `The admin command provides a convenient interface for managing GitSense Chat
configuration without needing to worry about paths or installation details.

Once GSC_HOME is set, you can manage your configuration from any location.`,
}

func init() {
	NativeCmd.AddCommand(adminCmd)
}
