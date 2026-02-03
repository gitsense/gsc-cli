/*
 * Component: Interactive Profile Wizard
 * Block-UUID: 227e2c90-fa4b-4d93-ab35-095e912b513c
 * Parent-UUID: 32a3a2d9-f843-4885-b7ae-681aca198f98
 * Version: 1.1.0
 * Description: Interactive wizards for creating, updating, and selecting context profiles using the survey library. Handles user prompts, validation, and confirmation steps. Updated to make Description and Aliases optional in the interactive prompts.
 * Language: Go
 * Created-at: 2026-02-03T05:45:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package manifest

import (
	"context"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/yourusername/gsc-cli/internal/registry"
)

// CreateProfileInteractive guides the user through creating a profile using survey prompts.
func CreateProfileInteractive(ctx context.Context, name string) error {
	// 1. Description (Optional)
	var description string
	promptDesc := &survey.Input{
		Message: "Description (Optional - Press Enter to skip):",
		Help:    "A brief description of this profile's purpose.",
	}
	if err := survey.AskOne(promptDesc, &description); err != nil {
		return err
	}

	// Auto-generate description if left blank for better UX in list views
	if description == "" {
		description = fmt.Sprintf("Context profile for %s", name)
	}

	// 2. Select Database
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	if len(reg.Databases) == 0 {
		return fmt.Errorf("no databases found. Please import a manifest first using 'gsc manifest import'")
	}

	var dbOptions []string
	for _, db := range reg.Databases {
		dbOptions = append(dbOptions, db.Name)
	}

	var selectedDB string
	promptDB := &survey.Select{
		Message: "Choose a Default Database:",
		Options: dbOptions,
	}
	if err := survey.AskOne(promptDB, &selectedDB); err != nil {
		return err
	}

	// 3. Select Field
	fieldNames, err := ListFieldNames(ctx, selectedDB)
	if err != nil {
		return fmt.Errorf("failed to list fields for database '%s': %w", selectedDB, err)
	}

	if len(fieldNames) == 0 {
		return fmt.Errorf("no fields found in database '%s'", selectedDB)
	}

	var selectedField string
	promptField := &survey.Select{
		Message: "Choose a Default Query Field:",
		Options: fieldNames,
	}
	if err := survey.AskOne(promptField, &selectedField); err != nil {
		return err
	}

	// 4. Aliases (Optional)
	var aliasesStr string
	promptAliases := &survey.Input{
		Message: "Aliases (Optional - Press Enter to skip):",
		Help:    "Short names to quickly switch to this profile (e.g., sec, audit).",
	}
	if err := survey.AskOne(promptAliases, &aliasesStr); err != nil {
		return err
	}

	var aliases []string
	if aliasesStr != "" {
		aliases = strings.Split(aliasesStr, ",")
		for i := range aliases {
			aliases[i] = strings.TrimSpace(aliases[i])
		}
	}

	// 5. Preview & Confirm
	fmt.Println("\n────────────────────────────────────────")
	fmt.Printf("Profile Summary:\n")
	fmt.Printf("  Name:        %s\n", name)
	fmt.Printf("  Description: %s\n", description)
	fmt.Printf("  Database:    %s\n", selectedDB)
	fmt.Printf("  Field:       %s\n", selectedField)
	fmt.Printf("  Aliases:     %s\n", strings.Join(aliases, ", "))
	fmt.Println("────────────────────────────────────────")

	var confirm bool
	promptConfirm := &survey.Confirm{
		Message: "Create this profile?",
		Default: true,
	}
	if err := survey.AskOne(promptConfirm, &confirm); err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("profile creation cancelled")
	}

	// 6. Create Profile
	settings := ProfileSettings{
		Global: GlobalSettings{DefaultDatabase: selectedDB},
		Query:  QuerySettings{DefaultField: selectedField, DefaultFormat: "table"},
		RG:     RGSettings{DefaultFormat: "table", DefaultContext: 0},
	}

	return CreateProfile(name, description, aliases, settings)
}

// UpdateProfileInteractive guides the user through updating a profile.
func UpdateProfileInteractive(ctx context.Context, name string) error {
	// 1. Load existing profile
	profile, err := LoadProfile(name)
	if err != nil {
		return err
	}

	// 2. Description (Optional)
	var description string
	promptDesc := &survey.Input{
		Message: "Description (Optional - Press Enter to keep current):",
		Default: profile.Description,
		Help:    "A brief description of this profile's purpose.",
	}
	if err := survey.AskOne(promptDesc, &description); err != nil {
		return err
	}

	// 3. Select Database
	reg, err := registry.LoadRegistry()
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	var dbOptions []string
	for _, db := range reg.Databases {
		dbOptions = append(dbOptions, db.Name)
	}

	var selectedDB string
	promptDB := &survey.Select{
		Message: "Choose a Default Database:",
		Options: dbOptions,
		Default: profile.Settings.Global.DefaultDatabase,
	}
	if err := survey.AskOne(promptDB, &selectedDB); err != nil {
		return err
	}

	// 4. Select Field
	fieldNames, err := ListFieldNames(ctx, selectedDB)
	if err != nil {
		return fmt.Errorf("failed to list fields for database '%s': %w", selectedDB, err)
	}

	var selectedField string
	promptField := &survey.Select{
		Message: "Choose a Default Query Field:",
		Options: fieldNames,
		Default: profile.Settings.Query.DefaultField,
	}
	if err := survey.AskOne(promptField, &selectedField); err != nil {
		return err
	}

	// 5. Manage Aliases (Optional)
	var aliasesStr string
	promptAliases := &survey.Input{
		Message: "Aliases (Optional - Press Enter to keep current):",
		Default: strings.Join(profile.Aliases, ", "),
		Help:    "Short names to quickly switch to this profile (e.g., sec, audit).",
	}
	if err := survey.AskOne(promptAliases, &aliasesStr); err != nil {
		return err
	}

	var aliases []string
	if aliasesStr != "" {
		aliases = strings.Split(aliasesStr, ",")
		for i := range aliases {
			aliases[i] = strings.TrimSpace(aliases[i])
		}
	}

	// 6. Preview & Confirm
	fmt.Println("\n────────────────────────────────────────")
	fmt.Printf("Changes for '%s':\n", name)
	if description != profile.Description {
		fmt.Printf("  Description: %s -> %s\n", profile.Description, description)
	} else {
		fmt.Printf("  Description: %s (unchanged)\n", profile.Description)
	}
	
	if selectedDB != profile.Settings.Global.DefaultDatabase {
		fmt.Printf("  Database:    %s -> %s\n", profile.Settings.Global.DefaultDatabase, selectedDB)
	} else {
		fmt.Printf("  Database:    %s (unchanged)\n", profile.Settings.Global.DefaultDatabase)
	}

	if selectedField != profile.Settings.Query.DefaultField {
		fmt.Printf("  Field:       %s -> %s\n", profile.Settings.Query.DefaultField, selectedField)
	} else {
		fmt.Printf("  Field:       %s (unchanged)\n", profile.Settings.Query.DefaultField)
	}

	fmt.Printf("  Aliases:     %s -> %s\n", strings.Join(profile.Aliases, ", "), strings.Join(aliases, ", "))
	fmt.Println("────────────────────────────────────────")

	var confirm bool
	promptConfirm := &survey.Confirm{
		Message: "Save these changes?",
		Default: true,
	}
	if err := survey.AskOne(promptConfirm, &confirm); err != nil {
		return err
	}

	if !confirm {
		return fmt.Errorf("profile update cancelled")
	}

	// 7. Update Profile
	profile.Description = description
	profile.Settings.Global.DefaultDatabase = selectedDB
	profile.Settings.Query.DefaultField = selectedField
	profile.Aliases = aliases

	return SaveProfile(profile)
}

// SelectProfileInteractive lets the user choose a profile from a list.
func SelectProfileInteractive() (*Profile, error) {
	profiles, err := ListProfiles()
	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found. Create one with 'gsc config context create <name>'")
	}

	var options []string
	profileMap := make(map[string]*Profile)

	for i := range profiles {
		label := profiles[i].Name
		if len(profiles[i].Aliases) > 0 {
			label += fmt.Sprintf(" (%s)", strings.Join(profiles[i].Aliases, ", "))
		}
		options = append(options, label)
		profileMap[label] = &profiles[i]
	}

	var selected string
	prompt := &survey.Select{
		Message: "Choose a profile:",
		Options: options,
	}

	if err := survey.AskOne(prompt, &selected); err != nil {
		return nil, err
	}

	return profileMap[selected], nil
}
