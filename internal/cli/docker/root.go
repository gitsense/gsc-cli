/**
 * Component: Docker CLI Root
 * Block-UUID: 7fd3c196-b698-4f4d-a991-d25b2606cd30
 * Parent-UUID: 4a9acd55-a824-43c4-8148-8125fe80bf1c
 * Version: 1.0.1
 * Description: Defines the root 'docker' command and registers subcommands for container management and host-side orchestration.
 * Language: Go
 * Created-at: 2026-03-19T17:19:13.241Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.0.1)
 */


package docker

import (
	"github.com/spf13/cobra"
)

// DockerCmd represents the base command for Docker container management
var DockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Manage the GitSense Chat Docker container and host-side orchestration",
	Long: `The docker command suite allows you to manage the lifecycle of the 
GitSense Chat container and provides a host-side watcher to enable 
native terminal and editor integration from within the container.`,
}

// RegisterCommand adds the docker command and its subcommands to the root CLI
func RegisterCommand(root *cobra.Command) {
	root.AddCommand(DockerCmd)
}

func init() {
	// Subcommands will be registered here by their respective files
	// using the init() function in start.go, watch.go, lifecycle.go, and install.go
}

