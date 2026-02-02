/*
 * Component: Version Information
 * Block-UUID: 42ce9290-86bc-45b0-9d43-c50e65ec517e
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Package version provides version information for the GSC CLI tool.
 * Language: Go
 * Created-at: 2026-02-02T05:46:05.908Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package version

import "fmt"

// These variables are set at build time using ldflags
var (
	// Version is the current version of the application
	Version = "1.0.0"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// BuildTime is the timestamp when the binary was built
	BuildTime = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
