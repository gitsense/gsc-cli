/**
 * Component: Auth Command Root
 * Block-UUID: a3322d4d-3c0c-4794-b457-4955668a88d7
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the root 'auth' command group for managing temporary authentication codes.
 * Language: Go
 * Created-at: 2026-05-20T15:55:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package auth

import (
	"github.com/spf13/cobra"
)

// AuthCmd represents the base command for auth code management
var AuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage temporary authentication codes",
	Long: `The auth command suite allows you to generate, validate, and delete 
temporary authentication codes for GitSense Chat operations.`,
}

// RegisterCommand adds the auth command and its subcommands to a parent command
func RegisterCommand(parent *cobra.Command) {
	parent.AddCommand(AuthCmd)
}

func init() {
	// Subcommands (generate, validate, delete) are registered by their respective files.
}
