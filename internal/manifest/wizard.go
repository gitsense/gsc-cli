/**
 * Component: Interactive Profile Wizard
 * Block-UUID: 1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d
 * Parent-UUID: eea72eed-99c5-4b58-81ea-fd50121f020d
 * Version: 1.5.0
 * Description: Interactive wizards for creating, updating, and selecting context profiles using the survey library. Handles user prompts, validation, and confirmation steps. Added prompts for Focus Scope configuration (include/exclude patterns) in create and update workflows.
 * Language: Go
 * Created-at: 2026-02-11T01:57:34.369Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), GLM-4.7 (v1.3.0), GLM-4.7 (v1.4.0), Gemini 3 Flash (v1.5.0)
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
		dbOptions = append(dbOptions, db.DatabaseLabel)
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
		resolvedDB, err := registry.ResolveDatabase(selectedDB) // ResolveDatabase handles matching by Label now
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

	// 4. Configure Focus Scope (Optional)
	var configureScope bool
	promptScope := &survey.Confirm{
		Message: "Do you want to define a Focus Scope (include/exclude patterns) for this profile?",
		Default: false,
		Help:    "Focus Scope filters the repository to a specific territory of interest (e.g., 'src/**' only).",
	}
	if err := survey.AskOne(promptScope, &configureScope); err != nil {
		return err
	}

	var scope *ScopeConfig
	if configureScope {
		var includeStr, excludeStr string

		promptInc := &survey.Input{
			Message: "Include patterns (comma-separated, e.g., src/**,lib/**):",
			Help:    "Files matching these patterns will be included. Leave empty to include all tracked files.",
		}
		if err := survey.AskOne(promptInc, &includeStr); err != nil {
			return err
		}

		promptExc := &survey.Input{
			Message: "Exclude patterns (comma-separated, e.g., test/**,vendor/**):",
			Help:    "Files matching these patterns will be excluded from the included set.",
		}
		if err := survey.AskOne(promptExc, &excludeStr); err != nil {
			return err
		}

		scope = &ScopeConfig{}
		if includeStr != "" {
			scope.Include = strings.Split(includeStr, ",")
			for i := range scope.Include {
				scope.Include[i] = strings.TrimSpace(scope.Include[i])
			}
		}
		if excludeStr != "" {
			scope.Exclude = strings.Split(excludeStr, ",")
			for i := range scope.Exclude {
				scope.Exclude[i] = strings.TrimSpace(scope.Exclude[i])
			}
		}
	}

	// 5. Aliases (Optional)
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

	// 6. Preview & Confirm
	fmt.Println("\n────────────────────────────────────────")
	fmt.Printf("Profile Summary:\n")
	fmt.Printf("  Name:        %s\n", name)
	fmt.Printf("  Description: %s\n", description)
	
	dbDisplay := selectedDB
	if dbDisplay == "" {
		dbDisplay = "(none)"
	}
	fmt.Printf("  Database:    %s\n", dbDisplay) // dbDisplay is the slug (DatabaseName)

	fieldDisplay := selectedField
	if fieldDisplay == "" {
		fieldDisplay = "(none)"
	}
	fmt.Printf("  Field:       %s\n", fieldDisplay)
	
	if scope != nil {
		fmt.Printf("  Scope Include: %s\n", strings.Join(scope.Include, ", "))
		fmt.Printf("  Scope Exclude: %s\n", strings.Join(scope.Exclude, ", "))
	} else {
		fmt.Printf("  Scope:       (default - all tracked files)\n")
	}
	
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

	// 7. Create Profile
	settings := ProfileSettings{
		Global: GlobalSettings{
			DefaultDatabase: selectedDB, // selectedDB is the slug (DatabaseName)
			Scope:           scope,
		},
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
		dbOptions = append(dbOptions, db.DatabaseLabel)
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
		resolvedDB, err := registry.ResolveDatabase(selectedDB) // ResolveDatabase handles matching by Label now
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

	// 5. Update Focus Scope (Optional)
	var updateScope bool
	promptScope := &survey.Confirm{
		Message: "Do you want to update the Focus Scope configuration?",
		Default: false,
	}
	if err := survey.AskOne(promptScope, &updateScope); err != nil {
		return err
	}

	var scope *ScopeConfig
	if updateScope {
		var includeStr, excludeStr string
		
		// Pre-fill with existing values if available
		defaultInc := ""
		defaultExc := ""
		if profile.Settings.Global.Scope != nil {
			defaultInc = strings.Join(profile.Settings.Global.Scope.Include, ",")
			defaultExc = strings.Join(profile.Settings.Global.Scope.Exclude, ",")
		}

		promptInc := &survey.Input{
			Message: "Include patterns (comma-separated):",
			Default: defaultInc,
			Help:    "Files matching these patterns will be included.",
		}
		if err := survey.AskOne(promptInc, &includeStr); err != nil {
			return err
		}

		promptExc := &survey.Input{
			Message: "Exclude patterns (comma-separated):",
			Default: defaultExc,
			Help:    "Files matching these patterns will be excluded.",
		}
		if err := survey.AskOne(promptExc, &excludeStr); err != nil {
			return err
		}

		scope = &ScopeConfig{}
		if includeStr != "" {
			scope.Include = strings.Split(includeStr, ",")
			for i := range scope.Include {
				scope.Include[i] = strings.TrimSpace(scope.Include[i])
			}
		}
		if excludeStr != "" {
			scope.Exclude = strings.Split(excludeStr, ",")
			for i := range scope.Exclude {
				scope.Exclude[i] = strings.TrimSpace(scope.Exclude[i])
			}
		}
	} else {
		// Keep existing scope
		scope = profile.Settings.Global.Scope
	}

	// 6. Manage Aliases (Optional)
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

	// 7. Preview & Confirm
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

	if updateScope {
		fmt.Printf("  Scope:       Updated\n")
		fmt.Printf("    Include:   %s\n", strings.Join(scope.Include, ", "))
		fmt.Printf("    Exclude:   %s\n", strings.Join(scope.Exclude, ", "))
	} else {
		fmt.Printf("  Scope:       (unchanged)\n")
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

	// 8. Update Profile
	profile.Description = description
	profile.Settings.Global.DefaultDatabase = selectedDB // selectedDB is the slug (DatabaseName)
	profile.Settings.Query.DefaultField = selectedField
	profile.Settings.Global.Scope = scope
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
		profileMap[label] = &profiles[i] // profiles[i].Name is the profile name, not the DB name
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
