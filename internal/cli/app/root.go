/*
 * Component: Unified App CLI Root
 * Block-UUID: 9a05bc6a-9922-498d-99a3-a7119cbefc10
 * Parent-UUID: 05bf2c4c-e94d-4f70-8980-eb91d1aa0d87
 * Version: 1.5.0
 * Description: Registered 'contract', 'ws', and 'exec' commands as subcommands of 'app' to consolidate application-specific interfaces.
 * Language: Go
 * Created-at: 2026-05-13T18:54:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), GLM-4.7 (v1.5.0)
 */


package app

import (
	"github.com/spf13/cobra"
	"github.com/gitsense/gsc-cli/internal/cli/app/native"
	"github.com/gitsense/gsc-cli/internal/cli/app/docker"
	importcmd "github.com/gitsense/gsc-cli/internal/cli/app/import"
	analysiscmd "github.com/gitsense/gsc-cli/internal/cli/app/analysis"
	authcmd "github.com/gitsense/gsc-cli/internal/cli/app/auth"
	"github.com/gitsense/gsc-cli/internal/cli/contract"
	"github.com/gitsense/gsc-cli/internal/cli/ws"
)

// AppCmd represents the unified app command for managing GitSense Chat
var AppCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage the GitSense Chat application",
	Long: `The app command provides a unified interface for managing the 
GitSense Chat application across different deployment modes and operations.

Available deployment modes:
  - native: Manage a native Node.js application running directly on the host
  - docker: Manage a Docker container running the GitSense Chat application
  - import: Import data sources into the GitSense Chat database
  - analysis: Manage metadata and analysis results
  - auth: Manage temporary authentication codes

Application operations:
  - contract: Manage traceability contracts between CLI and Chat
  - ws: Workspace management and entry
  - exec: Execute a command and send output to GitSense Chat`,
}

// RegisterCommand adds the app command and its deployment mode subcommands to the root CLI
func RegisterCommand(root *cobra.Command) {
	// Register deployment modes
	AppCmd.AddCommand(native.NativeCmd)
	AppCmd.AddCommand(docker.DockerCmd)
	
	// Register import command
	importcmd.RegisterCommand(AppCmd)
	
	// Register analysis command
	analysiscmd.RegisterCommand(AppCmd)
	
	// Register auth command
	authcmd.RegisterCommand(AppCmd)
	
	// Register application operations
	contract.RegisterContractCommand(AppCmd)
	ws.RegisterCommand(AppCmd)
	RegisterExecCommand(AppCmd)
	
	// Register app command to root
	root.AddCommand(AppCmd)
}
