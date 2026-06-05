/*
 * Component: Import Git Delete Shadow Command
 * Block-UUID: e6e1a8d9-93b3-4b05-b311-339cfa07faf9
 * Parent-UUID: e6be829f-3b36-4f82-acc2-3adbae9cab14
 * Version: 1.0.0
 * Description: Extracted shadow deletion workflow logic from command.go to separate the deletion orchestration from the standard import flow.
 * Language: Go
 * Created-at: 2026-05-17T15:10:02.000Z
 * Authors: GLM-4.7 (v1.0.0)
 */


package importgit

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

// runDeleteShadow handles the --delete-shadow flag logic
func runDeleteShadow(gscHome, gitPath, owner, repo, branch string, force bool) error {
	// Targeted mode: all identifiers provided
	if owner != "" && repo != "" && branch != "" {
		targetPath := ShadowPath(gscHome, owner, repo, branch)
		
		if !ShadowExists(targetPath) {
			return fmt.Errorf("shadow repository not found: %s", targetPath)
		}
		
		size, _ := ShadowSize(targetPath)
		fmt.Printf("Shadow repository found:\n")
		fmt.Printf("  Path: %s\n", targetPath)
		fmt.Printf("  Size: %s\n", formatBytes(size))
		
		if !force {
			confirm := false
			prompt := &survey.Confirm{
				Message: "Are you sure you want to delete this shadow repository?",
				Default: false,
			}
			if err := survey.AskOne(prompt, &confirm); err != nil {
				return err
			}
			if !confirm {
				fmt.Println("Deletion cancelled.")
				return nil
			}
		}
		
		if err := DeleteShadow(targetPath); err != nil {
			return err
		}
		
		// Clear from state if gitPath is known
		if gitPath != "" {
			clearShadowFromState(gitPath, branch)
		}
		
		fmt.Println("✓ Shadow repository deleted.")
		return nil
	}
	
	// Interactive mode: list and select
	shadows, err := ListShadows(gscHome)
	if err != nil {
		return fmt.Errorf("failed to list shadow repositories: %w", err)
	}
	
	if len(shadows) == 0 {
		fmt.Println("No shadow repositories found.")
		return nil
	}
	
	fmt.Println("\nAvailable shadow repositories:")
	options := make([]string, len(shadows))
	for i, s := range shadows {
		options[i] = fmt.Sprintf("%s/%s/%s (%s)", s.Owner, s.Repo, s.Branch, formatBytes(s.SizeBytes))
		fmt.Printf("  [%d] %s\n", i+1, options[i])
	}
	
	// FIX: Use string for survey.Select, not int
	var selectedOption string
	prompt := &survey.Select{
		Message: "Select shadow repository to delete:",
		Options: append(options, "Cancel"),
	}
	if err := survey.AskOne(prompt, &selectedOption); err != nil {
		return err
	}
	
	// FIX: Check for "Cancel" string, not index
	if selectedOption == "Cancel" {
		fmt.Println("Cancelled.")
		return nil
	}
	
	// FIX: Map selected string back to shadow index
	selected := -1
	for i, opt := range options {
		if opt == selectedOption {
			selected = i
			break
		}
	}
	
	// FIX: Validate selection
	if selected == -1 || selected >= len(shadows) {
		return fmt.Errorf("invalid selection")
	}
	
	target := shadows[selected]
	
	if !force {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Delete %s/%s/%s?", target.Owner, target.Repo, target.Branch),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Deletion cancelled.")
			return nil
		}
	}
	
	if err := DeleteShadow(target.Path); err != nil {
		return err
	}
	
	// Note: In interactive mode, we don't have the gitPath to clear state easily
	// User can re-import to clear state, or we could add a lookup mechanism
	
	fmt.Println("✓ Shadow repository deleted.")
	return nil
}
