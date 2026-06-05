//go:build windows

/**
 * Component: Daemon Windows
 * Block-UUID: 5f8314f3-4793-42fa-ac87-3defe9c36ff7
 * Parent-UUID: 50fb4bdd-4d21-4935-8ce1-e7888eed2d58
 * Version: 1.1.0
 * Description: Windows stub for Daemonize. Background daemon mode is not supported on Windows; directs users to the --foreground flag.
 * Language: Go
 * Created-at: 2026-05-12T01:15:20.129Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0)
 */


package app

import "fmt"

func Daemonize(opts SupervisorOptions) error {
	return fmt.Errorf("daemon mode is not supported on Windows; use --foreground flag")
}
