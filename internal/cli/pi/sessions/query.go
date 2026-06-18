/**
 * Component: Pi Sessions Query Command
 * Block-UUID: 28c28a9f-e833-4bdd-96dd-7a8c6ab25cf0
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements phase-one discovery queries over the Pi sessions SQLite mirror.
 * Language: Go
 * Created-at: 2026-06-18T00:00:00Z
 * Authors: Codex GPT-5 (v1.0.0)
 */

package sessions

import (
	"encoding/json"
	"fmt"
	"os"

	pisessions "github.com/gitsense/gsc-cli/internal/pi/sessions"
	"github.com/spf13/cobra"
)

// hiddenTextFlagName is the flag name for the hidden --text alias.
const hiddenTextFlagName = "text"

func queryCmd() *cobra.Command {
	var options pisessions.QueryOptions
	var dbPath string
	var format string

	cmd := &cobra.Command{
		Use:          "query",
		Short:        "Query the Pi sessions mirror",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedDB, err := resolvePiSessionsDBPath(dbPath)
			if err != nil {
				return err
			}
			options.DBPath = resolvedDB
			if options.Limit == 0 {
				options.Limit = 50
			}
			results, err := pisessions.Query(cmd.Context(), options)
			if err != nil {
				return err
			}
			return writeQueryResults(results, format)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite mirror path (default: GSC_HOME/data/pi-sessions.sqlite3)")
	cmd.Flags().StringVar(&options.File, "file", "", "Repo-root-relative file path to recall")
	cmd.Flags().StringVar(&options.AbsFile, "abs-file", "", "Absolute file path to recall")
	cmd.Flags().StringVar(&options.Repo, "repo", "", "Repo root filter")
	cmd.Flags().StringVar(&options.SessionID, "session-id", "", "Pi session ID filter")
	cmd.Flags().StringVar(&options.Tool, "tool", "", "Tool name filter (bash, read, edit, write)")
	cmd.Flags().StringVar(&options.Op, "op", "", "File operation filter (read, edit, write)")
	cmd.Flags().StringVarP(&options.Text, "message", "q", "", "Full-text search over user/assistant messages")
	cmd.Flags().StringVar(&options.Text, hiddenTextFlagName, "", "")
	cmd.Flags().MarkHidden(hiddenTextFlagName)
	cmd.Flags().StringVar(&options.CommandStartsWith, "command-starts-with", "", "Bash command prefix match (implies --tool bash)")
	cmd.Flags().StringVar(&options.CommandContains, "command-contains", "", "Bash command substring match (implies --tool bash)")
	cmd.Flags().StringVar(&options.OutputContains, "output-contains", "", "Tool output substring match")
	cmd.Flags().StringVar(&options.ToolArgsContains, "tool-args-contains", "", "Tool arguments JSON substring match")
	cmd.Flags().BoolVarP(&options.CaseInsensitive, "case-insensitive", "i", false, "Case-insensitive matching for --command-*, --output-*, --tool-args-*")
	cmd.Flags().StringVar(&options.Since, "since", "", "Inclusive lower timestamp bound")
	cmd.Flags().StringVar(&options.Until, "until", "", "Inclusive upper timestamp bound")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Provider filter")
	cmd.Flags().StringVar(&options.Model, "model", "", "Model filter")
	cmd.Flags().StringVar(&options.Type, "type", "", "Entry type filter")
	cmd.Flags().StringVar(&options.Role, "role", "", "Message role filter")
	cmd.Flags().StringVar(&options.EntryID, "entry", "", "Entry id filter")
	cmd.Flags().IntVar(&options.Limit, "limit", 50, "Maximum results")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format: human, json")
	return cmd
}

func writeQueryResults(results []pisessions.QueryResult, format string) error {
	switch format {
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(results)
	case "human", "":
		for _, result := range results {
			fmt.Printf("%s", result.Kind)
			if result.SessionID != "" {
				fmt.Printf(" %s", result.SessionID)
			}
			if result.Timestamp != "" {
				fmt.Printf(" %s", result.Timestamp)
			}
			if result.Command != "" {
				fmt.Printf(" cmd=%s", result.Command)
			} else if result.FilePathRel != "" {
				fmt.Printf(" %s", result.FilePathRel)
			} else if result.AbsPath != "" {
				fmt.Printf(" %s", result.AbsPath)
			}
			if result.ToolName != "" {
				fmt.Printf(" tool=%s", result.ToolName)
			}
			if result.Op != "" {
				fmt.Printf(" op=%s", result.Op)
			}
			if result.EntryID != "" {
				fmt.Printf(" entry=%s", result.EntryID)
			}
			if result.Text != "" {
				fmt.Printf("\n  %s", result.Text)
			}
			fmt.Println()
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
