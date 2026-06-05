//go:build !windows

/**
 * Component: Unix SysProcAttr Helper (shared)
 * Block-UUID: 2a9630fd-f234-4d0a-8d50-2ab3717712c6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Platform-specific helper returning a Unix SysProcAttr with Setsid=true to detach background workers from the parent session. Compiled only on non-Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0)
 */


package shared

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
