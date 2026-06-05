/**
 * Component: Docs About Command
 * Block-UUID: fad5edff-6971-4f39-8f5b-660c5cad5350
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Subcommand for 'gsc docs about' that displays the product overview document explaining what GitSense Chat is and why users should install it.
 * Language: Go
 * Created-at: 2026-05-30T02:58:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package docs

import (
	"github.com/spf13/cobra"
)

// NewAboutCmd creates and returns the 'gsc docs about' subcommand.
func NewAboutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "about",
		Short: "What is GitSense Chat and why use it?",
		Long: `Displays the product overview document that explains what GitSense Chat is, 
its core philosophy, and why users should install it.

This document is designed to answer "What is this?" questions from skeptical or 
curious users and their agents.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printDoc("about")
		},
	}

	return cmd
}
