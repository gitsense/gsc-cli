/**
 * Component: Agent Package Constants
 * Block-UUID: f28ed17c-9903-4199-ada4-df78365411e4
 * Parent-UUID: 35d247fc-af42-4bb3-aad8-fa6a626c10e3
 * Version: 1.2.0
 * Description: Generic constants for agent package including token size limits and file reading configuration.
 * Language: Go
 * Created-at: 2026-04-14T03:17:00.000Z
 * Authors: GLM-4.7 (v1.0.0), GLM-4.7 (v1.1.0), GLM-4.7 (v1.2.0)
 */


package agent

const (
	// maxTokenSize defines the maximum size for a single JSONL event (10MB)
	maxTokenSize = 10 * 1024 * 1024

	// defaultFileReadMaxTokens defines the default max tokens for file reading
	// Users can override by setting CLAUDE_CODE_FILE_READ_MAX_OUTPUT_TOKENS environment variable
	defaultFileReadMaxTokens = 15000
)
