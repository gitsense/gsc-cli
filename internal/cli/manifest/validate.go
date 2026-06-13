package manifest

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/manifest"
	"github.com/spf13/cobra"
)

var validateQuiet bool

var validateCmd = &cobra.Command{
	Use:   "validate <file>",
	Short: "Validate a manifest JSON file",
	Long: `Validate a manifest JSON file against the schema. Checks structure,
required fields, field types, and reference integrity.

Exit codes:
  0 = valid (may have warnings)
  1 = invalid (has errors)

Use --quiet to suppress warnings.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		result := manifest.ValidateManifestFile(args[0], validateQuiet)
		printValidationReport(args[0], result)
		if !result.Valid {
			return fmt.Errorf("validation failed with %d error(s)", len(result.Errors))
		}
		return nil
	},
}

func init() {
	validateCmd.Flags().BoolVar(&validateQuiet, "quiet", false, "Suppress warnings, only show errors")
}

func printValidationReport(path string, result manifest.ValidationResult) {
	fmt.Printf("gsc manifest validate: %s\n\n", path)

	if result.JSONValid {
		fmt.Println("  OK Valid JSON")
	}
	if result.SchemaVersion != "" {
		fmt.Printf("  OK Schema version: %s\n", result.SchemaVersion)
	}
	if result.Valid {
		fmt.Printf("  OK %d analyzer(s), %d field(s)\n", result.Summary.AnalyzerCount, result.Summary.FieldCount)
		fmt.Printf("  OK %d file record(s)\n", result.Summary.FileCount)
		fmt.Println("  OK All references resolved")
	}

	for _, validationError := range result.Errors {
		if validationError.Field == "" {
			fmt.Printf("  ERROR %s\n", validationError.Message)
			continue
		}
		fmt.Printf("  ERROR %s: %s\n", validationError.Field, validationError.Message)
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("  Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("    WARN %s\n", warning)
		}
	}

	fmt.Println()
	if result.Valid {
		fmt.Printf("  Result: VALID (%d warning(s))\n", len(result.Warnings))
		return
	}
	fmt.Printf("  Result: INVALID (%d error(s))\n", len(result.Errors))
}
