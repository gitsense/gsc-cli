/*
 * Component: Config Command
 * Block-UUID: 9f046149-e0e6-4ca6-aa73-a9cb6325cfbe
 * Parent-UUID: a9f275db-f202-4052-a43f-44b85f1bb25d
 * Version: 1.1.0
 * Description: CLI command definition for 'gsc config', managing context profiles and workspace settings. Added deactivate command.
 * Language: Go
 * Created-at: 2026-02-03T02:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/gsc-cli/internal/manifest"
	"github.com/yourusername/gsc-cli/internal/output"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage context profiles and workspace settings",
	Long: `Manage context profiles and workspace settings.
Profiles allow you to switch between different workspaces (e.g., security, payments)
with a single command.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// useCmd represents the 'config use' command
var useCmd = &cobra.Command{
	Use:   "use <profile-name>",
	Short: "Activate a context profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := manifest.SetActiveProfile(name); err != nil {
			return err
		}
		fmt.Printf("Switched to profile '%s'.\n", name)
		return nil
	},
}

// deactivateCmd represents the 'config deactivate' command
var deactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Deactivate the current profile (revert to global defaults)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := manifest.DeactivateProfile(); err != nil {
			return err
		}
		fmt.Println("Profile deactivated. Using global defaults.")
		return nil
	},
}

// contextCmd represents the 'config context' command group
var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage context profiles",
}

// contextListCmd represents the 'config context list' command
var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available context profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := manifest.ListProfiles()
		if err != nil {
			return err
		}

		activeName, err := manifest.GetActiveProfileName()
		if err != nil {
			return err
		}

		if len(profiles) == 0 {
			fmt.Println("No profiles found.")
			fmt.Println("Create one with 'gsc config context create <name>'.")
			return nil
		}

		// Format as table
		headers := []string{"Name", "Description", "Database"}
		var rows [][]string

		for _, p := range profiles {
			marker := " "
			if p.Name == activeName {
				marker = "*"
			}

			dbName := p.Settings.Global.DefaultDatabase
			if dbName == "" {
				dbName = "(none)"
			}

			rows = append(rows, []string{
				fmt.Sprintf("%s %s", marker, p.Name),
				p.Description,
				dbName,
			})
		}

		fmt.Print(output.FormatTable(headers, rows))
		return nil
	},
}

// contextCreateCmd represents the 'config context create' command
var (
	createDesc       string
	createDB         string
	createField      string
	createFormat     string
	createRGContext  int
)

var contextCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new context profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Build settings
		settings := manifest.ProfileSettings{
			Global: manifest.GlobalSettings{
				DefaultDatabase: createDB,
			},
			Query: manifest.QuerySettings{
				DefaultField:  createField,
				DefaultFormat: createFormat,
			},
			RG: manifest.RGSettings{
				DefaultFormat:  createFormat,
				DefaultContext: createRGContext,
			},
		}

		if err := manifest.CreateProfile(name, createDesc, settings); err != nil {
			return err
		}

		fmt.Printf("Profile '%s' created successfully.\n", name)
		fmt.Printf("Activate it with 'gsc config use %s'.\n", name)
		return nil
	},
}

// contextShowCmd represents the 'config context show' command
var contextShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a context profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		profile, err := manifest.ShowProfile(name)
		if err != nil {
			return err
		}

		fmt.Printf("Name:        %s\n", profile.Name)
		fmt.Printf("Description: %s\n", profile.Description)
		fmt.Println("\nSettings:")
		fmt.Printf("  Database:       %s\n", profile.Settings.Global.DefaultDatabase)
		fmt.Printf("  Query Field:    %s\n", profile.Settings.Query.DefaultField)
		fmt.Printf("  Query Format:   %s\n", profile.Settings.Query.DefaultFormat)
		fmt.Printf("  RG Format:      %s\n", profile.Settings.RG.DefaultFormat)
		fmt.Printf("  RG Context:     %d\n", profile.Settings.RG.DefaultContext)

		return nil
	},
}

// contextDeleteCmd represents the 'config context delete' command
var contextDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a context profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := manifest.DeleteProfile(name); err != nil {
			return err
		}
		fmt.Printf("Profile '%s' deleted.\n", name)
		return nil
	},
}

// currentContextCmd represents the 'config current-context' command
var currentContextShort bool

var currentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Show the currently active profile",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := manifest.GetActiveProfileName()
		if err != nil {
			return err
		}

		if currentContextShort {
			// Output only the name for shell prompts
			if name == "" {
				fmt.Println("none")
			} else {
				fmt.Println(name)
			}
		} else {
			// Human-readable output
			if name == "" {
				fmt.Println("No active profile.")
				fmt.Println("Set one with 'gsc config use <name>'.")
			} else {
				fmt.Printf("Active profile: %s\n", name)
			}
		}
		return nil
	},
}

func init() {
	// Register context subcommands
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextCreateCmd)
	contextCmd.AddCommand(contextShowCmd)
	contextCmd.AddCommand(contextDeleteCmd)

	// Register config subcommands
	configCmd.AddCommand(useCmd)
	configCmd.AddCommand(deactivateCmd)
	configCmd.AddCommand(contextCmd)
	configCmd.AddCommand(currentContextCmd)

	// Add flags for context create
	contextCreateCmd.Flags().StringVar(&createDesc, "description", "", "Description of the profile")
	contextCreateCmd.Flags().StringVar(&createDB, "db", "", "Default database for this profile")
	contextCreateCmd.Flags().StringVar(&createField, "field", "", "Default query field for this profile")
	contextCreateCmd.Flags().StringVar(&createFormat, "format", "table", "Default output format for this profile")
	contextCreateCmd.Flags().IntVar(&createRGContext, "rg-context", 0, "Default ripgrep context lines for this profile")

	// Add flags for current-context
	currentContextCmd.Flags().BoolVar(&currentContextShort, "short", false, "Output only the profile name (for shell prompts)")
}

// RegisterConfigCommand registers the config command with the root command.
func RegisterConfigCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(configCmd)
}
