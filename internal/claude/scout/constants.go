/**
 * Component: Scout Package Constants
 * Block-UUID: 35d247fc-af42-4bb3-aad8-fa6a626c10e3
 * Parent-UUID: 74107ded-56da-47a7-a456-17871caae043
 * Version: 1.1.0
 * Description: Package-level constants shared across Scout files
 * Language: Go
 * Created-at: 2026-04-14T03:17:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0)
 */


package scout

const (
	// maxTokenSize defines the maximum size for a single JSONL event (10MB)
	maxTokenSize = 10 * 1024 * 1024

	// defaultClaudeFileReadMaxTokens defines the default max tokens for Claude Code file reading
	// Users can override by setting CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS environment variable
	defaultClaudeFileReadMaxTokens = 15000
)

