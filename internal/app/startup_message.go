/**
 * Component: Startup Message Display
 * Block-UUID: 9373a44d-1b64-4478-a675-5e85162716ec
 * Parent-UUID: 09fa8611-7d49-4129-9aa2-1e37f951dc60
 * Version: 1.1.0
 * Description: Shared function for displaying enhanced startup messages with verification, crash history warnings, and troubleshooting tips. Used by both daemon mode and foreground mode to provide consistent user experience. Fixed import cycle by using FormatUptime from internal/app package.
 * Language: Go
 * Created-at: 2026-05-30T17:30:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package app

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// DisplayStartupMessage displays enhanced startup message with verification and crash warnings
// Parameters:
//   - dataDir: Path to the data directory
//   - port: The port the application is running on
//   - isRestart: Whether this is a restart operation (affects message text)
// Returns:
//   - error: Returns an error if the application failed to start
func DisplayStartupMessage(dataDir, port string, isRestart bool) error {
	// Show waiting message before pausing
	fmt.Print("  Verifying application startup...")
	time.Sleep(3 * time.Second)
	fmt.Println(" done.")

	// Check if process is still running
	running, _, childPid, err := IsProcessRunning(dataDir)
	if err != nil {
		fmt.Printf("  Warning: Failed to verify process status: %v\n", err)
	}

	// Load health status to check crash history
	health, _ := LoadHealthStatus(dataDir)

	// Determine the actual port (from health status or from parameter)
	actualPort := port
	if health != nil && health.Port != "" {
		actualPort = health.Port
	}

	envPath := filepath.Join(dataDir, ".env")

	if running {
		// Success message
		fmt.Println("")
		fmt.Println(strings.Repeat("━", 60))
		action := "started"
		if isRestart {
			action = "restarted"
		}
		fmt.Printf("  ✓ GitSense Chat %s (PID %d, port %s)\n", action, childPid, actualPort)
		fmt.Println(strings.Repeat("━", 60))
		fmt.Printf("  Stop with: gsc app native stop\n")
		fmt.Printf("  Open browser: http://localhost:%s\n", actualPort)
		fmt.Println("")
		fmt.Println("  Next steps:")
		fmt.Printf("    - View logs: gsc app native logs --follow\n")
		fmt.Printf("    - Check status: gsc app native status\n")

		// Show crash warning if there's a crash history
		if health != nil && health.CrashCount > 0 {
			fmt.Println("")
			fmt.Println(strings.Repeat("━", 60))
			fmt.Println("  ⚠️  Warning: This application has crashed recently")
			fmt.Println(strings.Repeat("━", 60))
			fmt.Printf("  Crashes: %d\n", health.CrashCount)
			if health.RestartCount > 0 {
				fmt.Printf("  Restarts: %d\n", health.RestartCount)
			}
			if health.LastCrashAt != nil {
				timeSinceCrash := time.Since(*health.LastCrashAt)
				fmt.Printf("  Last crash: %s (%s ago)\n", 
					health.LastCrashAt.Format("2006-01-02 15:04:05 MST"),
					FormatUptime(timeSinceCrash))
			}
			fmt.Println("")
			fmt.Println("  To investigate:")
			fmt.Printf("    - View logs: gsc app native logs --follow\n")
			fmt.Printf("    - Check status: gsc app native status\n")
			fmt.Printf("    - Verify .env: nano %s\n", envPath)
		}

		fmt.Println(strings.Repeat("━", 60))
		fmt.Println("")
		return nil
	} else {
		// Failure message with troubleshooting
		fmt.Println("")
		fmt.Println(strings.Repeat("━", 60))
		action := "start"
		if isRestart {
			action = "restart"
		}
		fmt.Printf("  ✗ Application failed to %s\n", action)
		fmt.Println(strings.Repeat("━", 60))
		fmt.Println("")
		fmt.Println("  Troubleshooting:")
		fmt.Printf("    1. View error logs: gsc app native logs\n")
		fmt.Printf("    2. Check recent crashes: gsc app native status\n")
		fmt.Printf("    3. Verify .env configuration: nano %s\n", envPath)
		fmt.Println("")
		fmt.Println("  Common issues:")
		fmt.Printf("    - Port %s already in use (try: gsc app native %s --port 3358)\n", actualPort, action)
		fmt.Println("    - Missing or invalid API keys in .env file")
		fmt.Println("    - Database corruption (try: gsc app native setup)")
		fmt.Println("")
		fmt.Println(strings.Repeat("━", 60))
		fmt.Println("")

		return fmt.Errorf("application failed to %s", action)
	}
}
