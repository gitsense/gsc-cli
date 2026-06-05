//go:build windows

/**
 * Component: Windows SysProcAttr Helper (shared)
 * Block-UUID: f7f66a6c-0ec4-4b80-aa6d-bd7ca7d04b03
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Windows stub returning nil for SysProcAttr - session detachment via Setsid is not supported on Windows. Compiled only on Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0)
 */


package shared

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
