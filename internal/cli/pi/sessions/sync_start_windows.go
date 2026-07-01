//go:build windows

/**
 * Component: Pi Sessions Sync Start Windows Stub
 * Block-UUID: [to-be-generated]
 * Parent-UUID: 2e36c783-d48f-407e-b5ae-e7ff9f674fa2
 * Version: 1.0.0
 * Description: Windows stub for Pi sessions sync daemonization. Detach mode is not supported on Windows.
 * Language: Go
 * Created-at: 2026-06-20T00:00:00Z
 * Authors: MiMo-v2.5-Pro (v1.0.0)
 */



package sessions

import (
	"fmt"

	"github.com/spf13/cobra"
)

func daemonizeSync(cmd *cobra.Command, sessionsDir, dbPath string) error {
	return fmt.Errorf("detached sync is not supported on Windows")
}
