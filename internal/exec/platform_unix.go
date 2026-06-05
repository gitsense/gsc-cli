/*
 * Component: Executor Platform Logic (Unix)
 * Block-UUID: 02f19991-e730-49b0-ab5e-9d5eb76cbdec
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Added getSignals helper to provide Unix-specific signal definitions.
 * Language: Go
 * Created-at: 2026-03-19T13:45:00.000Z
 * Authors: Gemini 3 Flash (v1.0.0), GLM-4.7 (v1.1.0)
 */


//go:build !windows

package exec

import (
	"os"
	"os/exec"
	"syscall"
)

// setProcessGroup configures the command to run in its own process group.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends a signal to the entire process group.
func killProcessGroup(cmd *exec.Cmd, sig os.Signal) {
	// Send signal to the negative PID to target the process group
	syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal))
}

// getSignals returns the list of signals to listen for on Unix systems.
func getSignals() []os.Signal {
	return []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP}
}
