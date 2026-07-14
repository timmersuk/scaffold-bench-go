package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestToolsReadWriteLsEdit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// write
	res, err := ExecuteTool(ctx, "write", `{"path":"foo.txt","content":"hello world"}`, dir, "call-1", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res != "created foo.txt" {
		t.Fatalf("unexpected write result: %s", res)
	}

	// read
	res, err = ExecuteTool(ctx, "read", `{"path":"foo.txt"}`, dir, "call-2", nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res != "hello world" {
		t.Fatalf("unexpected read result: %s", res)
	}

	// edit
	res, err = ExecuteTool(ctx, "edit", `{"path":"foo.txt","old_str":"hello","new_str":"goodbye"}`, dir, "call-3", nil)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if res != "ok" {
		t.Fatalf("unexpected edit result: %s", res)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "foo.txt"))
	if string(content) != "goodbye world" {
		t.Fatalf("unexpected content after edit: %s", string(content))
	}

	// ls
	res, err = ExecuteTool(ctx, "ls", `{"path":"."}`, dir, "call-4", nil)
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if res != `["foo.txt"]` {
		t.Fatalf("unexpected ls result: %s", res)
	}
}

func TestEditCreatesFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	res, err := ExecuteTool(ctx, "edit", `{"path":"bar/baz.txt","old_str":"","new_str":"new"}`, dir, "call-1", nil)
	if err != nil {
		t.Fatalf("edit create: %v", err)
	}
	if res != "created bar/baz.txt" {
		t.Fatalf("unexpected result: %s", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "bar", "baz.txt"))
	if string(data) != "new" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestToolPathEscapes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := ExecuteTool(ctx, "read", `{"path":"../secret.txt"}`, dir, "call-1", nil); err == nil {
		t.Fatal("expected escape error")
	}
	if _, err := ExecuteTool(ctx, "read", `{"path":"/etc/passwd"}`, dir, "call-2", nil); err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestUnknownTool(t *testing.T) {
	ctx := context.Background()
	if _, err := ExecuteTool(ctx, "nope", `{}`, t.TempDir(), "call-1", nil); err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestExecuteToolBatchSequential(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Write a file first
	_, err := ExecuteTool(ctx, "write", `{"path":"test.txt","content":"hello"}`, dir, "call-0", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	calls := []OpenAIToolCall{
		{ID: "call-1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
		{ID: "call-2", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
	}

	results, err := ExecuteToolBatch(ctx, calls, dir, ToolExecutionSequential, nil)
	if err != nil {
		t.Fatalf("ExecuteToolBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != "hello" || results[1] != "hello" {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestExecuteToolBatchParallel(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Write a file first
	_, err := ExecuteTool(ctx, "write", `{"path":"test.txt","content":"hello"}`, dir, "call-0", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	calls := []OpenAIToolCall{
		{ID: "call-1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
		{ID: "call-2", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
	}

	results, err := ExecuteToolBatch(ctx, calls, dir, ToolExecutionParallel, nil)
	if err != nil {
		t.Fatalf("ExecuteToolBatch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0] != "hello" || results[1] != "hello" {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestExecuteToolBatchParallelMixed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Write a file first
	_, err := ExecuteTool(ctx, "write", `{"path":"test.txt","content":"hello"}`, dir, "call-0", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	calls := []OpenAIToolCall{
		{ID: "call-1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
		{ID: "call-2", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "edit", Arguments: `{"path":"test.txt","old_str":"hello","new_str":"world"}`}},
		{ID: "call-3", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"path":"test.txt"}`}},
	}

	results, err := ExecuteToolBatch(ctx, calls, dir, ToolExecutionParallel, nil)
	if err != nil {
		t.Fatalf("ExecuteToolBatch: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// First read should see "hello"
	if results[0] != "hello" {
		t.Fatalf("expected first read to see 'hello', got %s", results[0])
	}
	// Edit should succeed
	if results[1] != "ok" {
		t.Fatalf("expected edit to succeed, got %s", results[1])
	}
	// Second read should see "world"
	if results[2] != "world" {
		t.Fatalf("expected second read to see 'world', got %s", results[2])
	}
}

func TestBeforeToolCallHook(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	hooks := &ToolExecutionHooks{
		BeforeToolCall: func(ctx context.Context, input BeforeToolCallInput) (*BeforeToolCallResult, error) {
			if input.Name == "write" {
				return &BeforeToolCallResult{Block: true, Reason: "writes not allowed"}, nil
			}
			return nil, nil
		},
	}

	// Try to write - should be blocked
	_, err := ExecuteTool(ctx, "write", `{"path":"test.txt","content":"hello"}`, dir, "call-1", hooks)
	if err == nil {
		t.Fatal("expected write to be blocked")
	}
	if err.Error() != "writes not allowed" {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to read - should succeed (file doesn't exist, but hook allows it)
	_, err = ExecuteTool(ctx, "read", `{"path":"test.txt"}`, dir, "call-2", hooks)
	if err == nil {
		t.Fatal("expected read to fail (file doesn't exist)")
	}
	// The error should be about file not found, not about blocking
	if err.Error() == "writes not allowed" {
		t.Fatal("read should not be blocked by hook")
	}
}

func TestAfterToolCallHook(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	hooks := &ToolExecutionHooks{
		AfterToolCall: func(ctx context.Context, input AfterToolCallInput) (*string, error) {
			// Override the result
			override := "OVERRIDDEN"
			return &override, nil
		},
	}

	// Write a file
	_, err := ExecuteTool(ctx, "write", `{"path":"test.txt","content":"hello"}`, dir, "call-1", nil)
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read with hook - result should be overridden
	result, err := ExecuteTool(ctx, "read", `{"path":"test.txt"}`, dir, "call-2", hooks)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if result != "OVERRIDDEN" {
		t.Fatalf("expected result to be overridden, got %s", result)
	}
}
