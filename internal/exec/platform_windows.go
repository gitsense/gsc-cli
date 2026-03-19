/*
 * Component: Executor Platform Logic (Windows)
 * Block-UUID: ec5df7b8-b520-4762-ab21-a9d2df74bcfd
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added getSignals helper to provide Windows-specific signal definitions.
 * Language: Go
 * Created-at: 2026-03-19T13:45:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


//go:build windows

package exec

import (
	"os"
)

// setProcessGroup is a no-op on Windows as Setpgid is not supported.
func setProcessGroup(cmd *exec.Cmd) {
	// Windows does not support Setpgid in SysProcAttr
}

// killProcessGroup sends a signal to the process. Windows does not support 
// Unix-style process groups via syscall.Kill.
func killProcessGroup(cmd *exec.Cmd, sig os.Signal) {
	cmd.Process.Signal(sig)
}

// getSignals returns the list of signals to listen for on Windows.
func getSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
