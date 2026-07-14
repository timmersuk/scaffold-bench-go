package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	outputCap                  = 8192
	defaultBashTimeoutMs       = 5000
	maxBashTimeoutMs           = 10000
	maxParallelSafeToolCalls   = 8
)

// ToolExecutionMode controls how tool batches are executed.
type ToolExecutionMode string

const (
	ToolExecutionSequential ToolExecutionMode = "sequential"
	ToolExecutionParallel   ToolExecutionMode = "parallel"
)

// BeforeToolCallInput is passed to the beforeToolCall hook.
type BeforeToolCallInput struct {
	ID         string
	Name       string
	RawArgs    string
	ParsedArgs any
	WorkDir    string
}

// BeforeToolCallResult is returned by the beforeToolCall hook.
type BeforeToolCallResult struct {
	Block  bool
	Reason string
}

// AfterToolCallInput is passed to the afterToolCall hook.
type AfterToolCallInput struct {
	BeforeToolCallInput
	Result string
}

// BeforeToolCallFunc is the signature for the beforeToolCall hook.
type BeforeToolCallFunc func(ctx context.Context, input BeforeToolCallInput) (*BeforeToolCallResult, error)

// AfterToolCallFunc is the signature for the afterToolCall hook.
type AfterToolCallFunc func(ctx context.Context, input AfterToolCallInput) (*string, error)

// ToolExecutionHooks configures middleware for tool execution.
type ToolExecutionHooks struct {
	BeforeToolCall BeforeToolCallFunc
	AfterToolCall  AfterToolCallFunc
}

// ToolDefinition describes one tool for the model.
type ToolDefinition struct {
	Type     string         `json:"type"`
	Function FunctionSchema `json:"function"`
}

// FunctionSchema is the JSON schema for a tool.
type FunctionSchema struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

// OpenAIToolCall is a tool call emitted by the model.
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ChatMessage is a message in the model conversation.
type ChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
}

type toolHandler struct {
	def ToolDefinition
	run func(ctx context.Context, args json.RawMessage, cwd string) (string, error)
}

// OpenAITools returns the tool schemas for the five supported tools.
func OpenAITools() []ToolDefinition {
	return []ToolDefinition{
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "read",
				Description: "Read the contents of a file at the given relative path. Returns file contents as a string.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Relative path to the file"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "ls",
				Description: "List files and directories at the given path. If no path is provided, lists the current directory. Directories are marked with a trailing slash.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string", "description": "Relative path to list (defaults to current directory)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name: "edit",
				Description: `Edit a file by replacing old_str with new_str.

Rules:
- old_str must match EXACTLY once in the file (including whitespace). If it appears zero or multiple times, the edit fails.
- If the file does not exist AND old_str is empty, the file is created with new_str as its contents.
- old_str and new_str must differ.`,
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"old_str": map[string]any{"type": "string"},
						"new_str": map[string]any{"type": "string"},
					},
					"required": []string{"path", "old_str", "new_str"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "write",
				Description: "Write a complete file at the given relative path. Creates parent directories if needed and overwrites existing contents.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":    map[string]any{"type": "string"},
						"content": map[string]any{"type": "string"},
					},
					"required": []string{"path", "content"},
				},
			},
		},
		{
			Type: "function",
			Function: FunctionSchema{
				Name:        "bash",
				Description: "Run a shell command with cwd set to the scenario working directory. Prefer this for fast codebase search and verification; prefer one focused search command over multiple tool calls.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{"type": "string"},
						"timeout_ms": map[string]any{"type": "number", "description": "Optional timeout in milliseconds (default 5000, max 10000)"},
					},
					"required": []string{"command"},
				},
			},
		},
	}
}

// ExecuteTool executes a single tool call with optional hooks and returns its string result.
func ExecuteTool(ctx context.Context, name string, rawArgs string, cwd string, callID string, hooks *ToolExecutionHooks) (string, error) {
	handler, ok := toolRegistry[name]
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}

	var parsedArgs any
	_ = json.Unmarshal(json.RawMessage(rawArgs), &parsedArgs)

	input := BeforeToolCallInput{
		ID:         callID,
		Name:       name,
		RawArgs:    rawArgs,
		ParsedArgs: parsedArgs,
		WorkDir:    cwd,
	}

	if hooks != nil && hooks.BeforeToolCall != nil {
		result, err := hooks.BeforeToolCall(ctx, input)
		if err != nil {
			return "", err
		}
		if result != nil && result.Block {
			reason := result.Reason
			if reason == "" {
				reason = "tool execution blocked"
			}
			return "", errors.New(reason)
		}
	}

	result, err := handler.run(ctx, json.RawMessage(rawArgs), cwd)

	if hooks != nil && hooks.AfterToolCall != nil {
		afterInput := AfterToolCallInput{
			BeforeToolCallInput: input,
			Result:              result,
		}
		override, hookErr := hooks.AfterToolCall(ctx, afterInput)
		if hookErr != nil {
			return "", hookErr
		}
		if override != nil {
			result = *override
		}
	}

	return result, err
}

// ExecuteToolBatch runs tool calls according to the execution mode and returns their results.
// In sequential mode, all calls run one at a time.
// In parallel mode, parallel-safe tools (read, ls) run concurrently, while mutating tools run sequentially.
func ExecuteToolBatch(ctx context.Context, calls []OpenAIToolCall, cwd string, mode ToolExecutionMode, hooks *ToolExecutionHooks) ([]string, error) {
	if mode != ToolExecutionParallel {
		results := make([]string, len(calls))
		for i, call := range calls {
			res, err := ExecuteTool(ctx, call.Function.Name, call.Function.Arguments, cwd, call.ID, hooks)
			if err != nil {
				res = fmt.Sprintf("error: %s", err.Error())
			}
			results[i] = res
		}
		return results, nil
	}

	results := make([]string, len(calls))
	i := 0
	for i < len(calls) {
		if !isParallelSafeTool(calls[i].Function.Name) {
			res, err := ExecuteTool(ctx, calls[i].Function.Name, calls[i].Function.Arguments, cwd, calls[i].ID, hooks)
			if err != nil {
				res = fmt.Sprintf("error: %s", err.Error())
			}
			results[i] = res
			i++
			continue
		}

		end := i + 1
		for end < len(calls) && isParallelSafeTool(calls[end].Function.Name) {
			end++
		}

		segment := calls[i:end]
		segmentResults := executeSafeBatchParallel(ctx, segment, cwd, hooks)
		for segIdx, res := range segmentResults {
			results[i+segIdx] = res
		}
		i = end
	}

	return results, nil
}

func isParallelSafeTool(name string) bool {
	return name == "read" || name == "ls"
}

func executeSafeBatchParallel(ctx context.Context, batch []OpenAIToolCall, cwd string, hooks *ToolExecutionHooks) []string {
	results := make([]string, len(batch))
	if len(batch) == 0 {
		return results
	}

	sem := make(chan struct{}, maxParallelSafeToolCalls)
	var wg sync.WaitGroup

	for i, call := range batch {
		wg.Add(1)
		go func(idx int, c OpenAIToolCall) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := ExecuteTool(ctx, c.Function.Name, c.Function.Arguments, cwd, c.ID, hooks)
			if err != nil {
				res = fmt.Sprintf("error: %s", err.Error())
			}
			results[idx] = res
		}(i, call)
	}

	wg.Wait()
	return results
}

var toolRegistry = map[string]toolHandler{
	"read": {
		def: OpenAITools()[0],
		run: func(_ context.Context, args json.RawMessage, cwd string) (string, error) {
			var a struct{ Path string `json:"path"` }
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			p, err := resolveToolPath(cwd, a.Path)
			if err != nil {
				return "", err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	},
	"ls": {
		def: OpenAITools()[1],
		run: func(_ context.Context, args json.RawMessage, cwd string) (string, error) {
			var a struct{ Path string `json:"path"` }
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			path := a.Path
			if path == "" {
				path = "."
			}
			p, err := resolveToolPath(cwd, path)
			if err != nil {
				return "", err
			}
			entries, err := os.ReadDir(p)
			if err != nil {
				return "", err
			}
			names := make([]string, len(entries))
			for i, e := range entries {
				if e.IsDir() {
					names[i] = e.Name() + "/"
				} else {
					names[i] = e.Name()
				}
			}
			b, _ := json.Marshal(names)
			return string(b), nil
		},
	},
	"edit": {
		def: OpenAITools()[2],
		run: func(_ context.Context, args json.RawMessage, cwd string) (string, error) {
			var a struct {
				Path   string `json:"path"`
				OldStr string `json:"old_str"`
				NewStr string `json:"new_str"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			if a.OldStr == a.NewStr {
				return "", errors.New("old_str and new_str are identical")
			}
			p, err := resolveToolPath(cwd, a.Path)
			if err != nil {
				return "", err
			}
			if _, err := os.Stat(p); os.IsNotExist(err) {
				if a.OldStr != "" {
					return "", fmt.Errorf("file not found: %s", a.Path)
				}
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return "", err
				}
				if err := os.WriteFile(p, []byte(a.NewStr), 0o644); err != nil {
					return "", err
				}
				return fmt.Sprintf("created %s", a.Path), nil
			} else if err != nil {
				return "", err
			}
			content, err := os.ReadFile(p)
			if err != nil {
				return "", err
			}
			matches := strings.Count(string(content), a.OldStr)
			if matches == 0 {
				return "", fmt.Errorf("old_str not found in %s", a.Path)
			}
			if matches > 1 {
				return "", fmt.Errorf("old_str appears %d times in %s; must be unique", matches, a.Path)
			}
			replaced := strings.Replace(string(content), a.OldStr, a.NewStr, 1)
			if err := os.WriteFile(p, []byte(replaced), 0o644); err != nil {
				return "", err
			}
			return "ok", nil
		},
	},
	"write": {
		def: OpenAITools()[3],
		run: func(_ context.Context, args json.RawMessage, cwd string) (string, error) {
			var a struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			p, err := resolveToolPath(cwd, a.Path)
			if err != nil {
				return "", err
			}
			existed := false
			if _, err := os.Stat(p); err == nil {
				existed = true
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
				return "", err
			}
			if existed {
				return fmt.Sprintf("updated %s", a.Path), nil
			}
			return fmt.Sprintf("created %s", a.Path), nil
		},
	},
	"bash": {
		def: OpenAITools()[4],
		run: func(ctx context.Context, args json.RawMessage, cwd string) (string, error) {
			var a struct {
				Command   string `json:"command"`
				TimeoutMs *int   `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(args, &a); err != nil {
				return "", err
			}
			timeoutMs := defaultBashTimeoutMs
			if a.TimeoutMs != nil {
				timeoutMs = *a.TimeoutMs
			}
			if timeoutMs < 1 {
				timeoutMs = 1
			}
			if timeoutMs > maxBashTimeoutMs {
				timeoutMs = maxBashTimeoutMs
			}
			timeout := time.Duration(timeoutMs) * time.Millisecond

			cmdCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				shell := os.Getenv("COMSPEC")
				if shell == "" {
					shell = "cmd.exe"
				}
				cmd = exec.CommandContext(cmdCtx, shell, "/c", a.Command)
			} else {
				shell := os.Getenv("SHELL")
				if shell == "" {
					shell = "/bin/sh"
				}
				cmd = exec.CommandContext(cmdCtx, shell, "-lc", a.Command)
			}
			cmd.Dir = cwd
			cmd.Env = bashEnv()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			timedOut := false
			err := cmd.Run()
			if cmdCtx.Err() == context.DeadlineExceeded {
				timedOut = true
			}
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				}
			}

			outStr := truncate(stdout.String())
			errStr := truncate(stderr.String())
			sections := []string{fmt.Sprintf("exit_code: %d", exitCode)}
			if strings.TrimSpace(outStr) != "" {
				sections = append(sections, fmt.Sprintf("stdout:\n%s", outStr))
			}
			if timedOut || strings.TrimSpace(errStr) != "" {
				if timedOut {
					errStr = fmt.Sprintf("%s\ntimed out after %dms", strings.TrimSpace(errStr), timeoutMs)
				}
				sections = append(sections, fmt.Sprintf("stderr:\n%s", errStr))
			}
			if len(sections) == 1 {
				sections = append(sections, "stdout:\n<empty>")
			}
			return strings.Join(sections, "\n\n"), nil
		},
	},
}

func resolveToolPath(cwd, relativePath string) (string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", errors.New("path is required")
	}
	if filepath.IsAbs(relativePath) {
		return "", errors.New("absolute paths are not allowed")
	}
	resolved := filepath.Join(cwd, relativePath)
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes working directory: %s", relativePath)
	}
	return resolved, nil
}

func bashEnv() []string {
	keys := []string{"PATH", "HOME", "SHELL", "USER", "LANG", "LC_ALL", "TERM"}
	if runtime.GOOS == "windows" {
		keys = []string{"PATH", "USERPROFILE", "SYSTEMROOT", "COMSPEC"}
	}
	var env []string
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return env
}

func truncate(value string) string {
	if len(value) > outputCap {
		return value[:outputCap] + "\n... [truncated]"
	}
	return value
}
