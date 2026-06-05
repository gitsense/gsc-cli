/**
 * Component: Docs Install Command
 * Block-UUID: 843e3ac6-89fe-4fc1-9b4f-e0da809586d1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs install' that displays the installation wizard document with comprehensive guidance for setting up the GitSense Chat web application.
 * Language: Go
 * Created-at: 2026-05-30T02:59:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewInstallCmd creates and returns the 'gsc docs install' subcommand.
func NewInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "How to install the GitSense Chat web application",
		Long: `Displays the installation wizard document that provides comprehensive guidance 
for installing the GitSense Chat web application (Native vs. Docker), including 
prerequisites, post-installation steps, and configuration.

This document includes embedded LLM Guidance that enables the AI to act as an 
installation expert, guiding users through the setup process with a wizard-like 
experience.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("install")
		},
	}

	return cmd
}
