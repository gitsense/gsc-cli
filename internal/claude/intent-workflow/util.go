/**
 * Component: Intent Workflow Utilities
 * Block-UUID: 9e98de26-3782-4ced-ae1c-70a927f08ce3
 * Parent-UUID: 11102500-2f2c-4288-9d68-2276e29ebcfa
 * Version: 1.1.0
 * Description: Shared utility helpers for the agent package
 * Language: Go
 * Created-at: 2026-04-23T18:59:16.286Z
 * Authors: Gemini 2.5 Flash Lite (v1.0.0), GLM-4.7 (v1.1.0)
 */


package intent_workflow

import (
	"os"
	"strings"
)


func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// parseTurnType decomposes a turn type like "resume-change" into
// baseType="change" and isResume=true. For standard types, isResume is false.
func parseTurnType(turnType string) (baseType string, isResume bool) {
	if strings.HasPrefix(turnType, "resume-") {
		return strings.TrimPrefix(turnType, "resume-"), true
	}
	return turnType, false
}

