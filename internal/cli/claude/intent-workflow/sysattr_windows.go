//go:build windows

/**
 * Component: Windows SysProcAttr Helper (intentworkflowcli)
 * Block-UUID: e7f7ad95-b131-4e0c-b330-7e72fa7cb2b1
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Windows stub returning nil for SysProcAttr - session detachment via Setsid is not supported on Windows. Compiled only on Windows targets.
 * Language: Go
 * Created-at: 2026-05-30T00:00:00.000Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0)
 */


package intentworkflowcli

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
