/**
 * Component: Rules Changelog Command
 * Block-UUID: f3a4b5c6-d7e8-9012-fabc-def012345678
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules changelog, the command for querying rule change history.
 * Language: Go
 * Created-at: 2026-06-21T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func changelogCmd() *cobra.Command {
	var (
		file       string
		id         string
		glob       string
		since      string
		format     string
		scopeValue string
	)
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Query changelog for rules",
		Long: `Query the changelog history for rules matching a file, glob, or rule ID.

Use --since to filter entries after a specific timestamp (RFC3339 format).

Exit codes:
  0 - Lookup succeeded (including "no changelog found")
  1 - Lookup failed (bad args, etc.)`,
		Example: `  # Get changelog for rules matching a file
  gsc rules changelog --file README.md

  # Get changelog for a specific rule
  gsc rules changelog --id rule_019ee812

  # Get changelog since a specific time
  gsc rules changelog --file README.md --since 2026-06-21T00:00:00Z

  # JSON output for agents
  gsc rules changelog --file README.md --format json

  # Query personal rules only
  gsc rules changelog --id rule_019ee812 --scope personal`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" && id == "" && glob == "" {
				return fmt.Errorf("at least one of --file, --id, or --glob is required")
			}

			scope, err := gitsensescope.ParseScope(scopeValue)
			if err != nil {
				return err
			}

			sourcedRecords, err := rulespkg.LoadRecordsFromScope(scope)
			if err != nil {
				return fmt.Errorf("failed to load rules: %w", err)
			}

			// Convert sourced records to plain rules for matching
			var records []rulespkg.Rule
			for _, sr := range sourcedRecords {
				records = append(records, sr.Rule)
			}

			var matched []rulespkg.MatchedRule
			queryType := ""
			queryValue := ""

			if file != "" {
				queryType = "file"
				queryValue = file
				matched = rulespkg.GetRulesForFileAllActions(records, file, "")
			} else if id != "" {
				queryType = "id"
				queryValue = id
				resolved, err := rulespkg.ResolveSourcedRecordFromRecords(id, sourcedRecords)
				if err != nil {
					return err
				}
				if resolved != nil {
					matched = []rulespkg.MatchedRule{
						{Rule: resolved.Rule, MatchReason: "id: " + id},
					}
				}
			} else if glob != "" {
				queryType = "glob"
				queryValue = glob
				matched = rulespkg.GetRulesForGlob(records, glob, "")
			}

			// Parse since timestamp
			var sinceTime time.Time
			if since != "" {
				sinceTime, err = time.Parse(time.RFC3339, since)
				if err != nil {
					return fmt.Errorf("invalid --since timestamp (use RFC3339): %w", err)
				}
			}

			switch format {
			case "json":
				return renderChangelogJSON(queryType, queryValue, matched, sinceTime)
			default:
				return renderChangelogHuman(queryType, queryValue, matched, sinceTime)
			}
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "File path to query")
	cmd.Flags().StringVar(&id, "id", "", "Rule ID to query")
	cmd.Flags().StringVar(&glob, "glob", "", "Glob pattern to query")
	cmd.Flags().StringVar(&since, "since", "", "Filter entries after this timestamp (RFC3339)")
	cmd.Flags().StringVarP(&format, "format", "o", "human", "Output format (human, json)")
	cmd.Flags().StringVar(&scopeValue, "scope", "all", "Read scope: all, repo, or personal")
	return cmd
}

func renderChangelogHuman(queryType, queryValue string, matched []rulespkg.MatchedRule, since time.Time) error {
	fmt.Printf("Query: %s=%s\n", queryType, queryValue)
	if !since.IsZero() {
		fmt.Printf("Since: %s\n", since.Format(time.RFC3339))
	}
	fmt.Printf("Rules matched: %d\n\n", len(matched))

	if len(matched) == 0 {
		fmt.Println("No rules found.")
		return nil
	}

	for _, mr := range matched {
		fmt.Printf("Rule: %s\n", mr.Rule.ID)
		fmt.Printf("Summary: %s\n", mr.Rule.Summary)

		if len(mr.Rule.Changelog) == 0 {
			fmt.Println("Changelog: (empty)")
		} else {
			fmt.Println("Changelog:")
			for _, entry := range mr.Rule.Changelog {
				if !since.IsZero() && entry.Timestamp.Before(since) {
					continue
				}
				fmt.Printf("  [%s] %s\n", entry.Timestamp.Format(time.RFC3339), entry.Message)
			}
		}
		fmt.Println()
	}
	return nil
}

func renderChangelogJSON(queryType, queryValue string, matched []rulespkg.MatchedRule, since time.Time) error {
	type changelogEntry struct {
		Timestamp time.Time `json:"timestamp"`
		Message   string    `json:"message"`
	}
	type ruleChangelog struct {
		ID        string           `json:"id"`
		Summary   string           `json:"summary"`
		Changelog []changelogEntry `json:"changelog"`
	}

	output := struct {
		Query struct {
			File string `json:"file,omitempty"`
			ID   string `json:"id,omitempty"`
			Glob string `json:"glob,omitempty"`
		} `json:"query"`
		Since string           `json:"since,omitempty"`
		Rules []ruleChangelog  `json:"rules"`
	}{}

	switch queryType {
	case "file":
		output.Query.File = queryValue
	case "id":
		output.Query.ID = queryValue
	case "glob":
		output.Query.Glob = queryValue
	}

	if !since.IsZero() {
		output.Since = since.Format(time.RFC3339)
	}

	for _, mr := range matched {
		rc := ruleChangelog{
			ID:      mr.Rule.ID,
			Summary: mr.Rule.Summary,
		}
		for _, entry := range mr.Rule.Changelog {
			if !since.IsZero() && entry.Timestamp.Before(since) {
				continue
			}
			rc.Changelog = append(rc.Changelog, changelogEntry{
				Timestamp: entry.Timestamp,
				Message:   entry.Message,
			})
		}
		output.Rules = append(output.Rules, rc)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
