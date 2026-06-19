/**
 * Component: Pi Sessions Sync Command
 * Block-UUID: c18409b8-dda6-4426-8b61-03eb43d1a1ce
 * Parent-UUID: N/A
 * Version: 1.1.0
 * Description: Defines shared sync flags, preserves one-shot import, and registers continuous sync lifecycle commands.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0, v1.1.0)
 */

package sessions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func syncCmd() *cobra.Command {
	return syncCmdWithDependencies(defaultSyncStartDependencies())
}

func syncCmdWithDependencies(startDependencies syncStartDependencies) *cobra.Command {
	config := &syncConfig{}
	var reset bool
	var yes bool
	var format string

	cmd := &cobra.Command{
		Use:          "sync",
		Short:        "One-shot import Pi session JSONL into the local mirror",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.sessionsDir == "" {
				return fmt.Errorf("--sessions-dir is required for one-shot sync")
			}
			resolvedDB, err := resolvePiSessionsDBPath(config.dbPath)
			if err != nil {
				return err
			}
			if reset {
				if !yes {
					if !term.IsTerminal(int(os.Stdin.Fd())) {
						return fmt.Errorf("--reset requires confirmation. Run with --yes for non-interactive use")
					}
					if !confirmReset(resolvedDB) {
						fmt.Println("Canceled. Database left unchanged.")
						return nil
					}
				}
				if err := pisessions.ResetDatabase(resolvedDB); err != nil {
					return err
				}
			}
			result, err := pisessions.Sync(cmd.Context(), pisessions.SyncOptions{
				SessionsDir: config.sessionsDir,
				DBPath:      resolvedDB,
			})
			if err != nil {
				return err
			}
			return writeSyncResult(result, format)
		},
	}
	cmd.PersistentFlags().StringVar(&config.sessionsDir, "sessions-dir", "", "Root directory containing Pi session JSONL files")
	cmd.PersistentFlags().StringVar(&config.dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi-sessions.sqlite3)")
	cmd.Flags().BoolVar(&reset, "reset", false, "Delete and recreate the mirror database before syncing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm destructive operations without prompting")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	cmd.AddCommand(syncStartCmd(config, startDependencies))
	return cmd
}

type syncConfig struct {
	sessionsDir string
	dbPath      string
}

func resolvePiSessionsDBPath(value string) (string, error) {
	if value != "" {
		return filepath.Abs(value)
	}
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return "", err
	}
	return settings.GetPiSessionsDatabasePath(gscHome), nil
}

func confirmReset(dbPath string) bool {
	fmt.Printf("Reset Pi sessions mirror database?\n\n  %s\n\nType 'reset' to continue: ", dbPath)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	return strings.TrimSpace(answer) == "reset"
}

func writeSyncResult(result pisessions.SyncResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "human", "":
		fmt.Printf("Synced Pi sessions into %s\n", result.DBPath)
		fmt.Printf("  Sessions dir: %s\n", result.SessionsDir)
		fmt.Printf("  Files scanned: %d\n", result.FilesScanned)
		fmt.Printf("  Sessions imported: %d\n", result.SessionsImported)
		fmt.Printf("  Messages: %d\n", result.MessagesImported)
		fmt.Printf("  Tool calls: %d\n", result.ToolCallsImported)
		fmt.Printf("  File refs: %d\n", result.FileRefsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, syncError := range result.Errors {
				fmt.Printf("    %s: %s\n", syncError.Path, syncError.Error)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
