//go:build windows

/**
 * Component: Windows SysProcAttr Helper (change)
 * Block-UUID: da8f64b2-066f-46a7-9270-ddfd05fb1f59
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Windows stub returning nil for SysProcAttr - session detachment via Setsid is not supported on Windows. Compiled only on Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package change

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
