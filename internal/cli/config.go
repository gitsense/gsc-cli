/*
 * Component: Config Command
 * Block-UUID: d227868b-0c31-4421-abec-93abef96c98d
 * Parent-UUID: 9f046149-e0e6-4ca6-aa73-a9cb6325cfbe
 * Version: 1.2.0
 * Description: CLI command definition for 'gsc config', managing context profiles and workspace settings. Added interactive modes for create, update, and use commands. Added alias support and confirmation prompts for deletion.
 * Language: Go
 * Created-at: 2026-02-03T02:10:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package cli

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
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
	Use:   "use [profile-name]",
	Short: "Activate a context profile",
	Long: `Activate a context profile by name or alias.
If no argument is provided, an interactive selection menu will appear.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Interactive selection
			profile, err := manifest.SelectProfileInteractive()
			if err != nil {
				return err
			}
			return manifest.SetActiveProfile(profile.Name)
		}

		// Direct lookup
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
		headers := []string{"Name", "Description", "Database", "Aliases"}
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

			aliases := strings.Join(p.Aliases, ", ")
			if aliases == "" {
				aliases = "-"
			}

			rows = append(rows, []string{
				fmt.Sprintf("%s %s", marker, p.Name),
				p.Description,
				dbName,
				aliases,
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
	createAliases    string
)

var contextCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new context profile",
	Long: `Create a new context profile.

If you provide all required flags (--db, --field), the profile is created immediately.
Otherwise, you'll be guided through an interactive setup wizard.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		ctx := cmd.Context()

		// Check if we have enough info for non-interactive mode
		hasDB := cmd.Flags().Changed("db")
		hasField := cmd.Flags().Changed("field")

		if hasDB && hasField {
			// Non-interactive mode
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

			var aliases []string
			if createAliases != "" {
				aliases = strings.Split(createAliases, ",")
				for i := range aliases {
					aliases[i] = strings.TrimSpace(aliases[i])
				}
			}

			if err := manifest.CreateProfile(name, createDesc, aliases, settings); err != nil {
				return err
			}
			fmt.Printf("Profile '%s' created successfully.\n", name)
			return nil
		}

		// Interactive mode
		return manifest.CreateProfileInteractive(ctx, name)
	},
}

// contextUpdateCmd represents the 'config context update' command
var (
	updateDesc    string
	updateDB      string
	updateField   string
	updateAliases string
)

var contextUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an existing context profile",
	Long: `Update an existing context profile.

If you provide update flags, the profile is updated immediately.
Otherwise, you'll be guided through an interactive update wizard.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		ctx := cmd.Context()

		// Check if any update flags are provided
		hasUpdates := cmd.Flags().Changed("description") ||
			cmd.Flags().Changed("db") ||
			cmd.Flags().Changed("field") ||
			cmd.Flags().Changed("alias")

		if hasUpdates {
			// Non-interactive mode
			profile, err := manifest.LoadProfile(name)
			if err != nil {
				return err
			}

			if updateDesc != "" {
				profile.Description = updateDesc
			}
			if updateDB != "" {
				profile.Settings.Global.DefaultDatabase = updateDB
			}
			if updateField != "" {
				profile.Settings.Query.DefaultField = updateField
			}
			if updateAliases != "" {
				aliases := strings.Split(updateAliases, ",")
				for i := range aliases {
					aliases[i] = strings.TrimSpace(aliases[i])
				}
				profile.Aliases = aliases
			}

			if err := manifest.SaveProfile(profile); err != nil {
				return err
			}
			fmt.Printf("Profile '%s' updated successfully.\n", name)
			return nil
		}

		// Interactive mode
		return manifest.UpdateProfileInteractive(ctx, name)
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
		if len(profile.Aliases) > 0 {
			fmt.Printf("Aliases:     %s\n", strings.Join(profile.Aliases, ", "))
		}
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

		// Confirmation prompt
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to delete '%s'?", name),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}

		if !confirm {
			fmt.Println("Deletion cancelled.")
			return nil
		}

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
	contextCmd.AddCommand(contextUpdateCmd)
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
	contextCreateCmd.Flags().StringVar(&createAliases, "alias", "", "Aliases for this profile (comma-separated)")

	// Add flags for context update
	contextUpdateCmd.Flags().StringVar(&updateDesc, "description", "", "Update description")
	contextUpdateCmd.Flags().StringVar(&updateDB, "db", "", "Update default database")
	contextUpdateCmd.Flags().StringVar(&updateField, "field", "", "Update default query field")
	contextUpdateCmd.Flags().StringVar(&updateAliases, "alias", "", "Update aliases (comma-separated)")

	// Add flags for current-context
	currentContextCmd.Flags().BoolVar(&currentContextShort, "short", false, "Output only the profile name (for shell prompts)")
}

// RegisterConfigCommand registers the config command with the root command.
func RegisterConfigCommand(rootCmd *cobra.Command) {
	rootCmd.AddCommand(configCmd)
}
