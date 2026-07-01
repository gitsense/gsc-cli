package rules

import (
	"fmt"

	"github.com/gitsense/gsc-cli/internal/gitsensescope"
	rulespkg "github.com/gitsense/gsc-cli/internal/rules"
)

func validateCreatorFlag(creator string) error {
	switch creator {
	case "", rulespkg.CreatorAgent, "human":
		return nil
	default:
		return fmt.Errorf("--creator must be one of: agent, human")
	}
}

func isAgentCreator(creator string) bool {
	return creator == rulespkg.CreatorAgent
}

func validateAndStripAgentChecklist(rule *rulespkg.Rule, target gitsensescope.Target) error {
	checklistErrors := rulespkg.ValidateAgentCreatorChecklist(*rule, target)
	if len(checklistErrors) > 0 {
		return agentChecklistError{errors: checklistErrors}
	}
	rule.CreatorChecklist = nil
	return nil
}

type agentChecklistError struct {
	errors []string
}

func (e agentChecklistError) Error() string {
	return "agent creator checklist is invalid"
}

func printAgentChecklistErrors(err error) {
	if checklistErr, ok := err.(agentChecklistError); ok {
		for _, item := range checklistErr.errors {
			fmt.Printf("  ERROR %s\n", item)
		}
	}
}
