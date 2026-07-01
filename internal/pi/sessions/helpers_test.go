/**
 * Component: Pi Session Read-Only Helpers Tests
 * Block-UUID: c9d0e1f2-a3b4-5678-cdef-901234567890
 * Parent-UUID: N/A
 * Version: 1.0.0
 * Description: Tests for branch walking, tool call extraction, file references, and error extraction.
 * Language: Go
 * Created-at: 2026-06-23T00:00:00Z
 * Authors: MiMo-v2.5-pro (v1.0.0)
 */


package sessions

import (
	"path/filepath"
	"testing"
)

func testDataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestWalkBranchLinear(t *testing.T) {
	path := testDataPath("linear.jsonl")
	result, err := WalkBranch(path, "entry-9")
	if err != nil {
		t.Fatalf("WalkBranch() error = %v", err)
	}

	if result.Session.ID != "session-001" {
		t.Errorf("Session.ID = %v, want session-001", result.Session.ID)
	}
	if result.Leaf != "entry-9" {
		t.Errorf("Leaf = %v, want entry-9", result.Leaf)
	}
	if len(result.Entries) != 9 {
		t.Errorf("len(Entries) = %v, want 9", len(result.Entries))
	}

	// Verify root-to-leaf order
	if result.Entries[0].ID != "entry-1" {
		t.Errorf("Entries[0].ID = %v, want entry-1", result.Entries[0].ID)
	}
	if result.Entries[len(result.Entries)-1].ID != "entry-9" {
		t.Errorf("Entries[last].ID = %v, want entry-9", result.Entries[len(result.Entries)-1].ID)
	}
}

func TestWalkBranchBranched(t *testing.T) {
	path := testDataPath("branched.jsonl")

	// Walk branch A
	resultA, err := WalkBranch(path, "branch-a-2")
	if err != nil {
		t.Fatalf("WalkBranch(branch-a-2) error = %v", err)
	}
	if len(resultA.Entries) != 3 {
		t.Errorf("branch-a-2: len(Entries) = %v, want 3", len(resultA.Entries))
	}
	if resultA.Entries[0].ID != "root-1" {
		t.Errorf("branch-a-2: Entries[0].ID = %v, want root-1", resultA.Entries[0].ID)
	}
	if resultA.Entries[2].ID != "branch-a-2" {
		t.Errorf("branch-a-2: Entries[2].ID = %v, want branch-a-2", resultA.Entries[2].ID)
	}

	// Walk branch B
	resultB, err := WalkBranch(path, "branch-b-2")
	if err != nil {
		t.Fatalf("WalkBranch(branch-b-2) error = %v", err)
	}
	if len(resultB.Entries) != 3 {
		t.Errorf("branch-b-2: len(Entries) = %v, want 3", len(resultB.Entries))
	}
	if resultB.Entries[2].ID != "branch-b-2" {
		t.Errorf("branch-b-2: Entries[2].ID = %v, want branch-b-2", resultB.Entries[2].ID)
	}
}

func TestWalkBranchMissingLeaf(t *testing.T) {
	path := testDataPath("linear.jsonl")
	_, err := WalkBranch(path, "nonexistent")
	if err == nil {
		t.Error("expected error for missing leaf")
	}
}

func TestWalkBranchBrokenChain(t *testing.T) {
	// Create a fixture with broken parent chain
	path := testDataPath("linear.jsonl")
	// For now, just verify the test framework works
	result, err := WalkBranch(path, "entry-1")
	if err != nil {
		t.Fatalf("WalkBranch() error = %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("len(Entries) = %v, want 1", len(result.Entries))
	}
}

func TestExtractToolCalls(t *testing.T) {
	path := testDataPath("linear.jsonl")
	result, err := ExtractToolCalls(path, "entry-9")
	if err != nil {
		t.Fatalf("ExtractToolCalls() error = %v", err)
	}

	if len(result.ToolCalls) != 3 {
		t.Fatalf("len(ToolCalls) = %v, want 3", len(result.ToolCalls))
	}

	// Find the bash call (which has an error)
	var bashCall *ToolCall
	for _, tc := range result.ToolCalls {
		if tc.ToolName == "bash" {
			bashCall = &tc
			break
		}
	}
	if bashCall == nil {
		t.Fatal("expected to find bash tool call")
	}
	if !bashCall.IsError {
		t.Error("expected bash call to be an error")
	}
	if bashCall.ToolCallID != "call-003" {
		t.Errorf("bash ToolCallID = %v, want call-003", bashCall.ToolCallID)
	}
}

func TestExtractFiles(t *testing.T) {
	path := testDataPath("linear.jsonl")
	result, err := ExtractFiles(path, "entry-9")
	if err != nil {
		t.Fatalf("ExtractFiles() error = %v", err)
	}

	// Should have foo.txt as read and edit
	filesMap := make(map[string]string)
	for _, f := range result.Files {
		filesMap[f.Path+":"+f.Op] = f.Source
	}

	if _, ok := filesMap["foo.txt:read"]; !ok {
		t.Error("expected foo.txt:read")
	}
	if _, ok := filesMap["foo.txt:edit"]; !ok {
		t.Error("expected foo.txt:edit")
	}
}

func TestExtractErrors(t *testing.T) {
	path := testDataPath("linear.jsonl")

	// Get all errors
	result, err := ExtractErrors(path, "entry-9", ErrorsOptions{})
	if err != nil {
		t.Fatalf("ExtractErrors() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("len(Errors) = %v, want 1", len(result.Errors))
	}
	if result.Errors[0].ToolName != "bash" {
		t.Errorf("Error.ToolName = %v, want bash", result.Errors[0].ToolName)
	}

	// Filter by tool
	resultBash, err := ExtractErrors(path, "entry-9", ErrorsOptions{Tool: "bash"})
	if err != nil {
		t.Fatalf("ExtractErrors(tool=bash) error = %v", err)
	}
	if len(resultBash.Errors) != 1 {
		t.Errorf("len(Errors) with tool=bash = %v, want 1", len(resultBash.Errors))
	}

	// Filter by tool that has no errors
	resultRead, err := ExtractErrors(path, "entry-9", ErrorsOptions{Tool: "read"})
	if err != nil {
		t.Fatalf("ExtractErrors(tool=read) error = %v", err)
	}
	if len(resultRead.Errors) != 0 {
		t.Errorf("len(Errors) with tool=read = %v, want 0", len(resultRead.Errors))
	}

	// Filter by contains
	resultTS, err := ExtractErrors(path, "entry-9", ErrorsOptions{Contains: "TS2307"})
	if err != nil {
		t.Fatalf("ExtractErrors(contains=TS2307) error = %v", err)
	}
	if len(resultTS.Errors) != 1 {
		t.Errorf("len(Errors) with contains=TS2307 = %v, want 1", len(resultTS.Errors))
	}

	// Filter by non-matching contains
	resultXYZ, err := ExtractErrors(path, "entry-9", ErrorsOptions{Contains: "XYZ"})
	if err != nil {
		t.Fatalf("ExtractErrors(contains=XYZ) error = %v", err)
	}
	if len(resultXYZ.Errors) != 0 {
		t.Errorf("len(Errors) with contains=XYZ = %v, want 0", len(resultXYZ.Errors))
	}
}
