/*
 * Component: Schema Command
 * Block-UUID: 8f639204-4518-4bd1-bb58-dc6576a2b181
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: CLI command for inspecting the schema of a manifest database, listing analyzers and their fields.
 * Language: Go
 * Created-at: 2026-02-02T07:56:00.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package manifest

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/output"
	"github.com/yourusername/gsc-cli/pkg/logger"
)

var schemaFormat string

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema <database-name>",
	Short: "Inspect the schema of a manifest database",
	Long: `Inspect the schema of a manifest database to see available analyzers 
and their associated metadata fields. This helps understand what data is available 
for querying.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dbName := args[0]

		logger.Info("Retrieving schema for database '%s'...", dbName)

		// Call the logic layer to get schema
		schema, err := manifest.GetSchema(cmd.Context(), dbName)
		if err != nil {
			logger.Error("Failed to retrieve schema: %v", err)
			return err
		}

		// Format and output the results
		switch schemaFormat {
		case "json":
			output.FormatJSON(schema)
		case "table":
			printSchemaTable(schema)
		case "csv":
			printSchemaCSV(schema)
		default:
			logger.Error("Unsupported format: %s", schemaFormat)
			return fmt.Errorf("unsupported format: %s", schemaFormat)
		}

		return nil
	},
}

func init() {
	// Add flags
	schemaCmd.Flags().StringVarP(&schemaFormat, "format", "f", "table", "Output format (json, table, csv)")
}

// printSchemaTable formats the schema as a text table
func printSchemaTable(schema *manifest.SchemaInfo) {
	if len(schema.Analyzers) == 0 {
		fmt.Println("No analyzers found in database.")
		return
	}

	headers := []string{"Analyzer Ref", "Analyzer Name", "Field Ref", "Field Name", "Type"}
	var rows [][]string

	for _, analyzer := range schema.Analyzers {
		if len(analyzer.Fields) == 0 {
			// Add a row for the analyzer even if it has no fields
			rows = append(rows, []string{analyzer.Ref, analyzer.Name, "", "", ""})
		} else {
			for _, field := range analyzer.Fields {
				rows = append(rows, []string{
					analyzer.Ref,
					analyzer.Name,
					field.Ref,
					field.Name,
					field.Type,
				})
			}
		}
	}

	fmt.Print(output.FormatTable(headers, rows))
}

// printSchemaCSV formats the schema as CSV
func printSchemaCSV(schema *manifest.SchemaInfo) {
	if len(schema.Analyzers) == 0 {
		fmt.Println("No analyzers found in database.")
		return
	}

	headers := []string{"Analyzer Ref", "Analyzer Name", "Field Ref", "Field Name", "Type"}
	var rows [][]string

	for _, analyzer := range schema.Analyzers {
		if len(analyzer.Fields) == 0 {
			rows = append(rows, []string{analyzer.Ref, analyzer.Name, "", "", ""})
		} else {
			for _, field := range analyzer.Fields {
				rows = append(rows, []string{
					analyzer.Ref,
					analyzer.Name,
					field.Ref,
					field.Name,
					field.Type,
				})
			}
		}
	}

	output.FormatCSV(headers, rows)
}

// GetSchemaCommand returns the schema command for registration
func GetSchemaCommand() *cobra.Command {
	return schemaCmd
}
