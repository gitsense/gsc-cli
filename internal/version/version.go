/**
 * Component: Version Information
 * Block-UUID: 0cf4b978-4325-4a48-9c8b-e6a0276ef598
 * Parent-UUID: 42ce9290-86bc-45b0-9d43-c50e65ec517e
 * Version: 1.0.1
 * Description: Package version provides version information for the GSC CLI tool.
 * Language: Go
 * Created-at: 2026-02-13T07:22:40.583Z
 * Authors: GLM-4.7 (v1.0.0), Gemini 3 Flash (v1.0.1)
 */


package version

import "fmt"

// These variables are set at build time using ldflags
var (
	// Version is the current version of the application
	Version = "0.1.0"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// BuildTime is the timestamp when the binary was built
	BuildTime = "unknown"
)

// GetVersion returns the full version string
func GetVersion() string {
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, GitCommit, BuildTime)
}
