/*
 * Component: Interactive Profile Wizard
 * Block-UUID: e8fa0910-ca7b-4764-9d00-6d43078d474d
 * Parent-UUID: 83298544-6ca6-4e03-b184-ef5b93cb5399
 * Version: 1.3.0
 * Description: Interactive wizards for creating, updating, and selecting context profiles using the survey library. Handles user prompts, validation, and confirmation steps. Updated to resolve database display names to physical names before saving to configuration.
 * Language: Go
 * Created-at: 2026-02-03T05:45:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0)
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
	dbOptions = append(dbOptions, "(Skip - No Default Database)") // NEW: Allow skipping
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

	// Handle Skip option
	if selectedDB == "(Skip - No Default Database)" {
		selectedDB = ""
	} else {
		// Resolve the display name to the physical database name
		resolvedDB, err := registry.ResolveDatabase(selectedDB)
		if err != nil {
			return fmt.Errorf("failed to resolve database '%s': %w", selectedDB, err)
		}
		selectedDB = resolvedDB
	}

	// 3. Select Field
	// Only ask for field if a database was selected, otherwise we can't list fields
	var selectedField string
	if selectedDB != "" {
		fieldNames, err := ListFieldNames(ctx, selectedDB)
		if err != nil {
			return fmt.Errorf("failed to list fields for database '%s': %w", selectedDB, err)
		}

		if len(fieldNames) == 0 {
			return fmt.Errorf("no fields found in database '%s'", selectedDB)
		}

		var fieldOptions []string
		fieldOptions = append(fieldOptions, "(Skip - No Default Field)") // NEW: Allow skipping
		for _, field := range fieldNames {
			fieldOptions = append(fieldOptions, field)
		}

		promptField := &survey.Select{
			Message: "Choose a Default Query Field:",
			Options: fieldOptions,
		}
		if err := survey.AskOne(promptField, &selectedField); err != nil {
			return err
		}

		// Handle Skip option
		if selectedField == "(Skip - No Default Field)" {
			selectedField = ""
		}
	} else {
		// If DB was skipped, Field must also be skipped
		selectedField = ""
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
	
	dbDisplay := selectedDB
	if dbDisplay == "" {
		dbDisplay = "(none)"
	}
	fmt.Printf("  Database:    %s\n", dbDisplay)

	fieldDisplay := selectedField
	if fieldDisplay == "" {
		fieldDisplay = "(none)"
	}
	fmt.Printf("  Field:       %s\n", fieldDisplay)
	
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
	dbOptions = append(dbOptions, "(Skip - No Default Database)") // NEW: Allow skipping
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

	// Handle Skip option
	if selectedDB == "(Skip - No Default Database)" {
		selectedDB = ""
	} else {
		// Resolve the display name to the physical database name
		resolvedDB, err := registry.ResolveDatabase(selectedDB)
		if err != nil {
			return fmt.Errorf("failed to resolve database '%s': %w", selectedDB, err)
		}
		selectedDB = resolvedDB
	}

	// 4. Select Field
	// Only ask for field if a database was selected, otherwise we can't list fields
	var selectedField string
	if selectedDB != "" {
		fieldNames, err := ListFieldNames(ctx, selectedDB)
		if err != nil {
			return fmt.Errorf("failed to list fields for database '%s': %w", selectedDB, err)
		}

		var fieldOptions []string
		fieldOptions = append(fieldOptions, "(Skip - No Default Field)") // NEW: Allow skipping
		for _, field := range fieldNames {
			fieldOptions = append(fieldOptions, field)
		}

		promptField := &survey.Select{
			Message: "Choose a Default Query Field:",
			Options: fieldOptions,
			Default: profile.Settings.Query.DefaultField,
		}
		if err := survey.AskOne(promptField, &selectedField); err != nil {
			return err
		}

		// Handle Skip option
		if selectedField == "(Skip - No Default Field)" {
			selectedField = ""
		}
	} else {
		// If DB was skipped, Field must also be skipped
		selectedField = ""
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
