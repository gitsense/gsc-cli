/**
 * Component: Experts Flags
 * Block-UUID: 2c99ef71-4e19-4356-94a8-d2c2c645d3b2
 * Parent-UUID: 879f1107-1113-44cf-82fb-7647ebd8b6b7
 * Version: 1.0.3
 * Description: Added Silent field to InitFlags to support silent mode for inline agents. When --silent is used, the experts-context.md file is generated without printing any output to stdout.
 * Language: Go
 * Created-at: 2026-05-01T16:40:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.0.1), GLM-4.7 (v1.0.2), GLM-4.7 (v1.0.3)
 */


package experts

import (
	"github.com/spf13/cobra"
)

// InitFlags holds the flag values for the 'gsc experts init' command.
type InitFlags struct {
	DBs       []string // Specific brain databases to include. Empty = all active.
	Force     bool     // Overwrite existing context file without prompting.
	UserLevel string   // Persona: "new" (Guide), "author" (Specialist), "user" (Consultant).
	Silent    bool     // Suppress all output (for inline agents).
}

// AddInitFlags adds the init-specific flags to the provided cobra command.
func AddInitFlags(cmd *cobra.Command, flags *InitFlags) {
	cmd.Flags().StringSliceVar(&flags.DBs, "db", []string{}, "Specific brain databases to include (default: all active)")
	cmd.Flags().BoolVarP(&flags.Force, "force", "f", false, "Overwrite existing context file")
	cmd.Flags().StringVar(&flags.UserLevel, "user-level", "user", "Persona: 'new' (Guide), 'author' (Specialist), 'user' (Consultant)")
	cmd.Flags().BoolVar(&flags.Silent, "silent", false, "Suppress all output (for inline agents)")
}

// StatusFlags holds the flag values for the 'gsc experts status' command.
// Currently empty but reserved for future extensions (e.g., --json output).
type StatusFlags struct{}

// AddStatusFlags adds the status-specific flags to the provided cobra command.
func AddStatusFlags(cmd *cobra.Command, flags *StatusFlags) {
	// No flags currently defined
}

// ForgetFlags holds the flag values for the 'gsc experts forget' command.
// Currently empty but reserved for future extensions (e.g., --force to skip confirmation).
type ForgetFlags struct{}

// AddForgetFlags adds the forget-specific flags to the provided cobra command.
func AddForgetFlags(cmd *cobra.Command, flags *ForgetFlags) {
	// No flags currently defined
}

// SetupAgentFlags holds the flag values for the 'gsc experts setup-agent' command.
type SetupAgentFlags struct {
	Force bool // Overwrite existing skill file without prompting.
}

// AddSetupAgentFlags adds the setup-agent-specific flags to the provided cobra command.
func AddSetupAgentFlags(cmd *cobra.Command, flags *SetupAgentFlags) {
	cmd.Flags().BoolVarP(&flags.Force, "force", "f", false, "Overwrite existing skill file")
}
