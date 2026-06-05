/*
 * Component: Context Models
 * Block-UUID: f27fef18-f972-4f0f-a438-754aad28e09f
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Defines the data structures for GitSense Chat context files, extracted from context messages.
 * Language: Go
 * Created-at: 2026-03-24T03:32:58.595Z
 * Authors: Gemini 3 Flash (v1.0.0)
 */


package context

// ContextFile represents a single file extracted from a context message.
type ContextFile struct {
	ChatID   int64  `json:"chat_id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Repo     string `json:"repo"`
	Size     int    `json:"size"`
	Tokens   int    `json:"tokens"`
	Content  string `json:"content"`
	Language string `json:"language"`
}
