//go:build !windows

/**
 * Component: Unix SysProcAttr Helper (change)
 * Block-UUID: d31cbd58-af03-4c35-b882-8a6c3efc14a6
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Platform-specific helper returning a Unix SysProcAttr with Setsid=true to detach resume worker processes from the parent session. Compiled only on non-Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package change

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
