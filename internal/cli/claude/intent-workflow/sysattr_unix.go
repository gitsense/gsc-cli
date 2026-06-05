//go:build !windows

/**
 * Component: Unix SysProcAttr Helper (intentworkflowcli)
 * Block-UUID: d82b6dd9-7f63-486b-8bbd-5312f0b6d007
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Platform-specific helper returning a Unix SysProcAttr with Setsid=true to detach retry worker processes from the parent session. Compiled only on non-Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0)
 */


package intentworkflowcli

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
