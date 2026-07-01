/**
 * Component: Topics Migrate Command
 * Block-UUID: a1b2c3d4-e5f6-7890-abcd-200000000011
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Migrates existing lessons, notes, and rules to the new topic format.
 * Language: Go
 * Created-at: 2026-06-22T10:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package topics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	lessonspkg "github.com/gitsense/gsc-cli/internal/lessons"
	notespkg "github.com/gitsense/gsc-cli/internal/notes"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	topicstopkg "github.com/gitsense/gsc-cli/internal/topics"
	"github.com/spf13/cobra"
)

// rawLesson is used to read the raw JSONL without normalization.
type rawLesson struct {
	Topic         string   `json:"topic"`
	AppliesTo     struct {
		Topics []string `json:"topics"`
	} `json:"applies_to"`
}

// rawRule is used to read the raw JSONL without normalization.
type rawRule struct {
	Topic     string `json:"topic"`
	AppliesTo struct {
		Topics []string `json:"topics"`
	} `json:"applies_to"`
}

func migrateCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate existing lessons, notes, and rules to the new topic format",
		Long: `Migrate existing knowledge records to use the new top-level topic field.

This command:
1. Reads all existing lessons, notes, and rules
2. Extracts topics from legacy applies_to.topics
3. Registers extracted topics in the topic registry (if not already registered)
4. Updates records to use the new top-level topic field

Use --dry-run to preview changes without applying them.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load existing topic registry
			registry, err := topicstopkg.LoadRegistry()
			if err != nil {
				return fmt.Errorf("failed to load topic registry: %w", err)
			}

			// Track topics to register
			topicsToRegister := map[string]bool{}

			// Load and analyze lessons (read raw to detect legacy topics)
			lessonsPath, err := lessonspkg.RecordsPath()
			if err != nil {
				return fmt.Errorf("failed to get lessons path: %w", err)
			}
			lessonsToMigrate := 0
			var rawLessons []rawLesson
			if file, err := os.Open(lessonsPath); err == nil {
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}
					var raw rawLesson
					if err := json.Unmarshal([]byte(line), &raw); err != nil {
						continue
					}
					rawLessons = append(rawLessons, raw)
					if raw.Topic == "" && len(raw.AppliesTo.Topics) > 0 {
						lessonsToMigrate++
						for _, t := range raw.AppliesTo.Topics {
							if !registry.Exists(t) {
								topicsToRegister[t] = true
							}
						}
					}
				}
				file.Close()
			}

			// Load and analyze notes
			_, err = notespkg.LoadRecords()
			if err != nil {
				return fmt.Errorf("failed to load notes: %w", err)
			}
			notesToMigrate := 0
			// Notes don't have legacy topics, but track for completeness

			// Load and analyze rules (read raw to detect legacy topics)
			rulesPath, err := rulespkg.RecordsPath()
			if err != nil {
				return fmt.Errorf("failed to get rules path: %w", err)
			}
			rulesToMigrate := 0
			var rawRules []rawRule
			if file, err := os.Open(rulesPath); err == nil {
				scanner := bufio.NewScanner(file)
				for scanner.Scan() {
					line := scanner.Text()
					if line == "" {
						continue
					}
					var raw rawRule
					if err := json.Unmarshal([]byte(line), &raw); err != nil {
						continue
					}
					rawRules = append(rawRules, raw)
					if raw.Topic == "" && len(raw.AppliesTo.Topics) > 0 {
						rulesToMigrate++
						for _, t := range raw.AppliesTo.Topics {
							if !registry.Exists(t) {
								topicsToRegister[t] = true
							}
						}
					}
				}
				file.Close()
			}

			// Summary
			fmt.Printf("Migration Summary:\n")
			fmt.Printf("  Lessons to migrate: %d\n", lessonsToMigrate)
			fmt.Printf("  Notes to migrate: %d\n", notesToMigrate)
			fmt.Printf("  Rules to migrate: %d\n", rulesToMigrate)
			fmt.Printf("  Topics to register: %d\n", len(topicsToRegister))

			if len(topicsToRegister) > 0 {
				fmt.Printf("\nTopics to register:\n")
				for t := range topicsToRegister {
					fmt.Printf("  - %s\n", t)
				}
			}

			if dryRun {
				fmt.Printf("\nDry run - no changes applied.\n")
				return nil
			}

			// Register new topics
			if len(topicsToRegister) > 0 {
				now := time.Now().UTC()
				for slug := range topicsToRegister {
					topic := topicstopkg.Topic{
						Slug:        slug,
						Description: fmt.Sprintf("Auto-migrated topic: %s", slug),
						CreatedAt:   now,
						UpdatedAt:   now,
					}
					if err := topicstopkg.AppendRecord(topic); err != nil {
						return fmt.Errorf("failed to register topic %q: %w", slug, err)
					}
					fmt.Printf("Registered topic: %s\n", slug)
				}
			}

			// Migrate lessons
			if lessonsToMigrate > 0 {
				lessons, err := lessonspkg.LoadRecords()
				if err != nil {
					return fmt.Errorf("failed to load lessons: %w", err)
				}
				// Re-read raw to get the original topics
				for i, raw := range rawLessons {
					if i < len(lessons) && raw.Topic == "" && len(raw.AppliesTo.Topics) > 0 {
						lessons[i].Topic = raw.AppliesTo.Topics[0]
						if len(raw.AppliesTo.Topics) > 1 {
							lessons[i].RelatedTopics = raw.AppliesTo.Topics[1:min(3, len(raw.AppliesTo.Topics))]
						}
						lessons[i].AppliesTo.Topics = nil
					}
				}
				if err := lessonspkg.WriteRecords(lessons); err != nil {
					return fmt.Errorf("failed to write lessons: %w", err)
				}
				fmt.Printf("Migrated %d lessons\n", lessonsToMigrate)
			}

			// Migrate rules
			if rulesToMigrate > 0 {
				rules, err := rulespkg.LoadRecords()
				if err != nil {
					return fmt.Errorf("failed to load rules: %w", err)
				}
				// Re-read raw to get the original topics
				for i, raw := range rawRules {
					if i < len(rules) && raw.Topic == "" && len(raw.AppliesTo.Topics) > 0 {
						rules[i].Topic = raw.AppliesTo.Topics[0]
						if len(raw.AppliesTo.Topics) > 1 {
							rules[i].RelatedTopics = raw.AppliesTo.Topics[1:min(3, len(raw.AppliesTo.Topics))]
						}
						rules[i].AppliesTo.Topics = nil
					}
				}
				if err := rulespkg.WriteRecords(rules); err != nil {
					return fmt.Errorf("failed to write rules: %w", err)
				}
				fmt.Printf("Migrated %d rules\n", rulesToMigrate)
			}

			fmt.Printf("\nMigration complete!\n")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")
	return cmd
}
