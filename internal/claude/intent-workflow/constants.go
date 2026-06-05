/**
 * Component: Intent Workflow Package Constants
 * Block-UUID: 3d129e6f-1922-4f76-85c2-4eaa6d006c73
 * Parent-UUID: 13f7eb3a-c485-41f8-825f-3ec71d908cb7
 * Version: 1.4.0
 * Description: Generic constants for agent package including token size limits and file reading configuration. Simplified model specification for correction turns to use family names directly instead of hardcoded model IDs. The Claude CLI tool now handles mapping family names to latest versions.
 * Language: Go
 * Created-at: 2026-04-21T22:44:56.322Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0), Gemini 2.5 Flash Lite (v1.3.0), GLM-4.7 (v1.4.0)
 */


package intent_workflow

import "fmt"

const (
	// maxTokenSize defines the maximum size for a single JSONL event (10MB)
	maxTokenSize = 10 * 1024 * 1024

	// defaultFileReadMaxTokens defines the default max tokens for file reading
	// Users can override by setting CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS environment variable
	defaultFileReadMaxTokens = 15000
)

// ModelFamily represents a Claude model family used for correction turns.
type ModelFamily string

const (
	ModelFamilyHaiku  ModelFamily = "haiku"
	ModelFamilySonnet ModelFamily = "sonnet"
	ModelFamilyOpus   ModelFamily = "opus"
)

// DefaultCorrectionModel is the default model family for correction turns.
const DefaultCorrectionModel = ModelFamilyHaiku

// DefaultCorrectionTries is the default number of correction attempts before
// failing a turn with a format error.
const DefaultCorrectionTries = 3

// GetModelID returns the model family name for a given model family.
// The Claude CLI tool handles mapping family names to the latest model versions.
// Returns an error if the family is not recognised.
func GetModelID(family ModelFamily) (string, error) {
	switch family {
	case ModelFamilyHaiku, ModelFamilySonnet, ModelFamilyOpus:
		return string(family), nil
	default:
		return "", fmt.Errorf("invalid model family %q (must be haiku, sonnet, or opus)", family)
	}
}

// ValidateModelFamily reports whether the given string is a valid model family
// name. Returns nil on success.
func ValidateModelFamily(family string) error {
	switch ModelFamily(family) {
	case ModelFamilyHaiku, ModelFamilySonnet, ModelFamilyOpus:
		return nil
	default:
		return fmt.Errorf("invalid model family %q (must be haiku, sonnet, or opus)", family)
	}
}
