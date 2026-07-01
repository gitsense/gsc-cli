/**
 * Component: Rules Trigger Template Command
 * Block-UUID: 3c4d5e6f-7a8b-9012-cdef-234567890123
 * Parent-UUID: N/A
 * Version: 4.1.0
 * Description: Implements gsc rules trigger template for printing trigger templates (V1 executable trigger contract with lifecycle events).
 * Language: Go
 * Created-at: 2026-06-22T00:00:00Z
 * Updated-at: 2026-06-24T12:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0), MiMo-v2.5-pro (v2.0.0), MiMo-v2.5-pro (v3.0.0), MiMo-v2.5-pro (v4.0.0), terrchen (v4.1.0)
 * Changelog:
 *   v4.1.0 - Add lifecycle event context to trigger templates
 */


package rules

import (
	"fmt"

	"github.com/spf13/cobra"
)

const triggerMinimalTemplate = `// Tool-trigger template for GitSense rules (V1 executable trigger contract with lifecycle events)
// This trigger receives context on stdin and returns a JSON result on stdout.
//
// Input context (V1 with lifecycle events):
//   - version: "1"
//   - event: { name, runtime, runtimeEvent }  // Canonical lifecycle event
//   - session: { id, path, cwd }
//   - conversation: { leafId, messageIds }
//   - model?: { provider, id, thinkingLevel }
//   - toolCall: { id, toolName, action, file?, command?, input }
//   - repo?: { root, normalizedFile? }
//   - rule: { id, summary, type, ruleHash, triggerHash, event }
//
// Canonical lifecycle events:
//   - session_start: Session initialization
//   - user_prompt_submit: User prompt received
//   - pre_tool_use: Before tool execution (default)
//   - post_tool_use: After tool execution
//   - post_tool_batch: After batch of tools
//   - stop: Session stopping
//   - session_end: Session cleanup
//
// Output (V1):
//   - matched: boolean (required) - does this rule apply?
//   - block: boolean (required) - should this tool call stop?
//   - message: string (required when block=true) - model-facing reason
//   - notice: string (optional) - user/operator-facing, never sent to LLM
//
// Action values:
//   - read: file read operation
//   - write: file write operation
//   - edit: file edit operation
//   - bash: bash command execution
//   - tool: generic tool (non-file, non-bash)
//   - mcp_tool: MCP-specific tool
//   - prompt: user prompt submission
//   - agent_end: agent completion
//
// Filter fields:
//   - tool_filter: glob pattern for tool name matching (e.g., "github.*")
//   - command_filter: regex pattern for bash command matching (e.g., "rm -rf|chmod -R")
//   - command_filter_ignore_case: case-insensitive command matching (default: false)

const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

// Example: Block edits to accounting files
const isEdit = ctx.toolCall?.action === "edit";
const isAccountingFile = ctx.repo?.normalizedFile?.startsWith("data/accounting/");
const applies = isEdit && isAccountingFile;

console.log(JSON.stringify({
    matched: applies,
    block: applies,
    message: applies
        ? "Before editing accounting files, read the accounting format guide."
        : undefined,
    notice: applies
        ? "Blocked: accounting guidance required before editing."
        : undefined
}));
`

const triggerFullTemplate = `// Tool-trigger template for GitSense rules (full V1 contract with lifecycle events)
// This template demonstrates all available context fields and output options.
//
// Execution:
//   - cwd = repo.root when repo is known, otherwise session.cwd
//   - stdin receives the context JSON
//   - stdout must be exactly one JSON object
//   - stderr is captured for status/errors only

const chunks = [];
for await (const chunk of process.stdin) chunks.push(chunk);
const ctx = JSON.parse(Buffer.concat(chunks).toString("utf8"));

// Lifecycle event information
const eventName = ctx.event?.name;        // Canonical event name (e.g., "pre_tool_use")
const eventRuntime = ctx.event?.runtime;  // Runtime identifier (e.g., "pi", "claude")
const runtimeEvent = ctx.event?.runtimeEvent; // Runtime-specific event name

// Event-specific payloads
// For user_prompt_submit and before_agent_start:
const promptText = ctx.payload?.prompt?.text;  // The user's prompt text
const promptImages = ctx.payload?.prompt?.images;  // Images in the prompt
const promptSource = ctx.payload?.prompt?.source;  // "interactive" or "api"

// For post_tool_use:
const toolResult = ctx.payload?.toolResult;  // Tool execution result
const toolOutput = ctx.payload?.toolResult?.output;  // Tool output
const toolError = ctx.payload?.toolResult?.error;  // Tool error

// For stop:
const lastMessage = ctx.payload?.stop?.lastMessage;  // Last assistant message
const changedFiles = ctx.payload?.stop?.changedFiles;  // Files changed in session

// For session_start/session_end:
const sessionReason = ctx.payload?.session?.reason;  // "startup", "new", "resume", "fork"

// Session information
const sessionId = ctx.session?.id;
const sessionPath = ctx.session?.path;
const sessionCwd = ctx.session?.cwd;

// Conversation state
const leafId = ctx.conversation?.leafId;
const messageIds = ctx.conversation?.messageIds || [];

// Model information (may be null)
const modelProvider = ctx.model?.provider;
const modelId = ctx.model?.id;
const thinkingLevel = ctx.model?.thinkingLevel;

// Tool call details (for pre_tool_use and post_tool_use events)
const toolCallId = ctx.toolCall?.id;
const toolName = ctx.toolCall?.toolName;
const action = ctx.toolCall?.action;   // read, write, edit, bash, tool, mcp_tool, prompt, agent_end, custom
const file = ctx.toolCall?.file;       // absolute path for file tools, null for bash
const command = ctx.toolCall?.command; // raw command for bash, null for file tools
const input = ctx.toolCall?.input;     // raw tool input

// Repository context (may be null if outside repo)
const repoRoot = ctx.repo?.root;
const normalizedFile = ctx.repo?.normalizedFile; // repo-relative path

// Rule reference
const ruleId = ctx.rule?.id;
const ruleSummary = ctx.rule?.summary;
const ruleType = ctx.rule?.type;
const ruleHash = ctx.rule?.ruleHash;
const triggerHash = ctx.rule?.triggerHash;
const ruleEvent = ctx.rule?.event;  // The event this rule is bound to

// Example: Switch on event name
switch (eventName) {
    case "pre_tool_use":
        // Inspect ctx.toolCall
        break;
    case "agent_end":
        // Inspect agent end state
        break;
    case "session_start":
        // Load initial context
        break;
    // ... handle other events
}

// Determine if this rule applies to the current event
const applies = false; // Replace with your logic

// If matched, determine if we should block
const shouldBlock = applies && false; // Replace with your logic

const message = "";  // Model-facing: block reason (required when block=true)
const notice = "";   // User-facing: shown in UI, never sent to LLM

console.log(JSON.stringify({
    matched: applies,
    block: shouldBlock,
    message: (applies && shouldBlock && message) ? message : undefined,
    notice: (applies && notice) ? notice : undefined
}));
`

func triggerTemplateCmd() *cobra.Command {
	var (
		full bool
	)

	cmd := &cobra.Command{
		Use:   "template",
		Short: "Print a trigger template",
		Long: `Print a JavaScript trigger template that can be used as a starting point
for creating tool-trigger rules.

V1 Executable Trigger Contract:
  - Context is passed on stdin (JSON)
  - stdout must return exactly one JSON object
  - stderr is captured for status/errors only
  - cwd = repo.root when repo is known, otherwise session.cwd

Output fields:
  - matched: boolean (required) - does this rule apply?
  - block: boolean (required) - should this tool call stop?
  - message: string (required when block=true) - model-facing reason
  - notice: string (optional) - user/operator-facing, never sent to LLM`,
		Example: `  # Print minimal template
  gsc rules trigger template

  # Print full template with all context fields
  gsc rules trigger template --full

  # Save template to file
  gsc rules trigger template > .gitsense/rules/triggers/my-trigger.mjs`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if full {
				fmt.Print(triggerFullTemplate)
			} else {
				fmt.Print(triggerMinimalTemplate)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Print full template with all context fields")

	return cmd
}
