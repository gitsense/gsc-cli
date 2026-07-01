/**
 * Component: Pi Sessions Sync Status Command
 * Block-UUID: [to-be-generated]
 * Parent-UUID: c18409b8-dda6-4426-8b61-03eb43d1a1ce
 * Version: 1.0.0
 * Description: Displays the current status of the Pi sessions sync watcher including PID, uptime, and recent sync activity.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package sessions

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	app "github.com/gitsense/gsc-cli/internal/app"
	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/gitsense/gsc-cli/pkg/settings"
	"github.com/spf13/cobra"
)

type syncStatusResult struct {
	Status     string           `json:"status"`
	PID        *int             `json:"pid,omitempty"`
	StartedAt  *string          `json:"started_at,omitempty"`
	Uptime     *string          `json:"uptime,omitempty"`
	SessionsDir string          `json:"sessions_dir"`
	Database   string           `json:"database"`
	LogFile    string           `json:"log_file"`
	DebugLogFile string         `json:"debug_log_file"`
	LastSync   *string          `json:"last_sync,omitempty"`
	Stats      *syncStats       `json:"stats,omitempty"`
	RecentSyncs []recentSync    `json:"recent_syncs,omitempty"`
	RecentErrors []recentError  `json:"recent_errors,omitempty"`
}

type syncStats struct {
	SessionsImported int `json:"sessions_imported"`
	MessagesImported int `json:"messages_imported"`
	ToolCallsImported int `json:"tool_calls_imported"`
}

type recentSync struct {
	Timestamp    string `json:"timestamp"`
	SessionFile  string `json:"session_file"`
	Delta        string `json:"delta"`
}

type recentError struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

func syncStatusCmd(config *syncConfig) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show the current status of the Pi sessions sync watcher",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncStatus(cmd, config, format)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

func runSyncStatus(cmd *cobra.Command, config *syncConfig, format string) error {
	gscHome, err := settings.GetGSCHome(false)
	if err != nil {
		return fmt.Errorf("resolve GSC_HOME: %w", err)
	}

	piDataDir := settings.GetPiGscDataDir(gscHome)
	pidPath := settings.GetPiSyncPIDPath(gscHome)
	logPath := settings.GetPiSyncLogPath(gscHome)
	debugLogPath := settings.GetPiSyncDebugLogPath(gscHome)
	dbPath, err := resolvePiSessionsDBPath(config.dbPath)
	if err != nil {
		return err
	}

	// Check process status
	running, supervisorPid, _, err := app.IsProcessRunning(piDataDir)
	if err != nil {
		return fmt.Errorf("check process status: %w", err)
	}

	result := syncStatusResult{
		SessionsDir:   resolveSessionsDirDisplay(config.sessionsDir),
		Database:      dbPath,
		LogFile:       logPath,
		DebugLogFile:  debugLogPath,
	}

	if running {
		result.Status = "running"
		result.PID = &supervisorPid

		// Try to get start time from PID file modification time
		if info, err := os.Stat(pidPath); err == nil {
			startedAt := info.ModTime().Format("2006-01-02 15:04:05")
			result.StartedAt = &startedAt
			uptime := time.Since(info.ModTime())
			uptimeStr := formatDuration(uptime)
			result.Uptime = &uptimeStr
		}
	} else {
		result.Status = "stopped"
	}

	// Get stats from database
	stats, lastSync, err := getSyncStats(dbPath)
	if err == nil {
		result.Stats = stats
		result.LastSync = lastSync
	}

	// Get recent syncs from database
	recentSyncs, err := getRecentSyncs(dbPath, 5)
	if err == nil {
		result.RecentSyncs = recentSyncs
	}

	// Get recent errors from log file
	recentErrors, err := getRecentErrors(logPath, 5)
	if err == nil {
		result.RecentErrors = recentErrors
	}

	return writeSyncStatusResult(cmd, result, format)
}

func resolveSessionsDirDisplay(value string) string {
	if value != "" {
		return value
	}
	return "~/.pi/agent/sessions"
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours < 24 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	days := hours / 24
	hours = hours % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

func getSyncStats(dbPath string) (*syncStats, *string, error) {
	// Use read-only connection
	db, err := pisessions.OpenQueryMirror(dbPath)
	if err != nil {
		return nil, nil, err
	}
	defer db.Close()

	stats := &syncStats{}
	var lastSync *string

	// Get aggregate stats
	err = db.QueryRow(`
		SELECT 
			COALESCE(SUM(message_count), 0),
			COALESCE(SUM(tool_call_count), 0),
			COUNT(*)
		FROM pi_chats WHERE file_deleted_at IS NULL
	`).Scan(&stats.MessagesImported, &stats.ToolCallsImported, &stats.SessionsImported)
	if err != nil {
		return nil, nil, err
	}

	// Get last sync time
	var lastSyncStr string
	err = db.QueryRow(`
		SELECT MAX(last_synced_at) FROM pi_chats WHERE last_synced_at IS NOT NULL
	`).Scan(&lastSyncStr)
	if err == nil && lastSyncStr != "" {
		lastSync = &lastSyncStr
	}

	return stats, lastSync, nil
}

func getRecentSyncs(dbPath string, limit int) ([]recentSync, error) {
	db, err := pisessions.OpenQueryMirror(dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT last_synced_at, session_file, message_count
		FROM pi_chats
		WHERE last_synced_at IS NOT NULL AND file_deleted_at IS NULL
		ORDER BY last_synced_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var syncs []recentSync
	for rows.Next() {
		var s recentSync
		var messageCount int
		var syncedAt time.Time
		err := rows.Scan(&syncedAt, &s.SessionFile, &messageCount)
		if err != nil {
			continue
		}

		// Truncate timestamp to seconds
		s.Timestamp = syncedAt.Format("2006-01-02T15:04:05Z")

		// Simplify session file display
		parts := strings.Split(s.SessionFile, "/")
		s.SessionFile = parts[len(parts)-1]

		s.Delta = fmt.Sprintf("%d messages", messageCount)

		syncs = append(syncs, s)
	}

	return syncs, nil
}

func getRecentErrors(logPath string, limit int) ([]recentError, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	// Read all lines and take the last N
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Take last N lines
	start := 0
	if len(lines) > limit {
		start = len(lines) - limit
	}

	var errors []recentError
	for _, line := range lines[start:] {
		// Parse log format: "2026-06-18T16:45:12Z WARN message"
		parts := strings.SplitN(line, " ", 3)
		if len(parts) >= 3 {
			errors = append(errors, recentError{
				Timestamp: parts[0],
				Level:     parts[1],
				Message:   parts[2],
			})
		}
	}

	return errors, nil
}

func writeSyncStatusResult(cmd *cobra.Command, result syncStatusResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	case "human", "":
		return writeSyncStatusHuman(cmd, result)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeSyncStatusHuman(cmd *cobra.Command, result syncStatusResult) error {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "\nPi Sessions Sync\n")
	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")
	fmt.Fprintf(out, "  Status:        %s\n", result.Status)

	if result.PID != nil {
		fmt.Fprintf(out, "  PID:           %d\n", *result.PID)
	} else {
		fmt.Fprintf(out, "  PID:           not running\n")
	}

	if result.StartedAt != nil {
		fmt.Fprintf(out, "  Started:       %s (%s ago)\n", *result.StartedAt, *result.Uptime)
	}

	fmt.Fprintf(out, "\n")
	fmt.Fprintf(out, "  Sessions Dir:  %s\n", result.SessionsDir)
	fmt.Fprintf(out, "  Database:      %s\n", result.Database)
	fmt.Fprintf(out, "  Log File:      %s\n", result.LogFile)
	fmt.Fprintf(out, "  Debug Log:     %s\n", result.DebugLogFile)

	if result.LastSync != nil {
		lastSync, err := time.Parse(time.RFC3339Nano, *result.LastSync)
		if err == nil {
			ago := formatDuration(time.Since(lastSync))
			fmt.Fprintf(out, "\n  Last Sync:     %s (%s ago)\n", lastSync.Format("2006-01-02 15:04:05"), ago)
		}
	}

	if result.Stats != nil {
		fmt.Fprintf(out, "  Sessions:      %d imported, %d messages, %d tool calls\n",
			result.Stats.SessionsImported,
			result.Stats.MessagesImported,
			result.Stats.ToolCallsImported)
	}

	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")

	// Recent syncs
	fmt.Fprintf(out, "\nRecent Syncs (last 5)\n")
	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")
	if len(result.RecentSyncs) == 0 {
		fmt.Fprintf(out, "  (none)\n")
	} else {
		for _, s := range result.RecentSyncs {
			fmt.Fprintf(out, "  %-20s  %-30s  %s\n", s.Timestamp, s.SessionFile, s.Delta)
		}
	}
	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")

	// Recent errors
	fmt.Fprintf(out, "\nRecent Errors (last 5)\n")
	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")
	if len(result.RecentErrors) == 0 {
		fmt.Fprintf(out, "  (none)\n")
	} else {
		for _, e := range result.RecentErrors {
			fmt.Fprintf(out, "  %-20s  %-6s  %s\n", e.Timestamp, e.Level, e.Message)
		}
	}
	fmt.Fprintf(out, "─────────────────────────────────────────────────\n")
	fmt.Fprintf(out, "\n")

	return nil
}
