/**
 * Component: Rules Execute Command
 * Block-UUID: c3d4e5f6-a7b8-9012-cdef-012345678901
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Implements gsc rules execute, the command for executing matched rules against a context. Runs triggers in parallel and returns execution results.
 * Language: Go
 * Created-at: 2026-06-26T16:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
	"github.com/spf13/cobra"
)

func executeCmd() *cobra.Command {
	var (
		contextFile string
		rulesFile   string
		format      string
		concurrency int
		timeout     time.Duration
	)
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute matched rules against a context",
		Long: `Execute matched rules against an execution context, running triggers in parallel
and returning the execution result.

This command takes the output of 'gsc rules get --format rules-json' and a V1ExecutionContext
JSON file, then executes triggers and builds the matched-rule packet.

Exit codes:
  0 - Evaluation completed successfully (block true/false is in JSON output)
  1 - Invalid input, runtime failure, or internal error`,
		Example: `  # Execute rules with context
  gsc rules execute --context ctx.json --rules rules.json

  # Execute with custom concurrency and timeout
  gsc rules execute --context ctx.json --rules rules.json -j 4 --timeout 10s

  # Pipe rules from gsc rules get
  gsc rules get --event pre_tool_use --action bash --command "rm -rf" --format rules-json | \
    gsc rules execute --context ctx.json --rules -`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read context file
			contextData, err := readFileOrStdin(contextFile)
			if err != nil {
				return fmt.Errorf("failed to read context file: %w", err)
			}

			// Parse context
			var execCtx rulespkg.V1ExecutionContext
			if err := json.Unmarshal(contextData, &execCtx); err != nil {
				return fmt.Errorf("invalid context JSON: %w", err)
			}

			// Validate context
			if execCtx.Version == "" {
				return fmt.Errorf("context version is required")
			}
			if execCtx.Event.Name == "" {
				return fmt.Errorf("context event.name is required")
			}

			// Read rules file
			rulesData, err := readFileOrStdin(rulesFile)
			if err != nil {
				return fmt.Errorf("failed to read rules file: %w", err)
			}

			// Parse rules
			var input rulespkg.RulesInput
			if err := json.Unmarshal(rulesData, &input); err != nil {
				return fmt.Errorf("invalid rules JSON: %w", err)
			}

			// Validate rules
			if input.SchemaVersion == 0 {
				return fmt.Errorf("rules schemaVersion is required")
			}

			// Execute rules
			ctx := context.Background()
			opts := rulespkg.ExecuteOptions{
				Concurrency: concurrency,
				Timeout:     timeout,
			}

			result, err := rulespkg.ExecuteRules(ctx, &input, &execCtx, opts)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			// Output result
			switch format {
			case "json":
				data, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal result: %w", err)
				}
				fmt.Println(string(data))
			default:
				return fmt.Errorf("unsupported format %q (use json)", format)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&contextFile, "context", "", "V1ExecutionContext JSON file (required)")
	cmd.Flags().StringVar(&rulesFile, "rules", "", "Rules JSON file from 'gsc rules get --format rules-json' (required, use '-' for stdin)")
	cmd.Flags().StringVarP(&format, "format", "o", "json", "Output format (json)")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 8, "Max parallel trigger executions")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Total execution budget (e.g., 10s, 500ms). 0 = no limit")
	cmd.MarkFlagRequired("context")
	cmd.MarkFlagRequired("rules")
	return cmd
}

// readFileOrStdin reads from a file path or stdin if the path is "-".
func readFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
