package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// Evaluator evaluates a scenario manifest against a workspace.
type Evaluator struct{}

// NewEvaluator returns a new Evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate runs all rubric checks in the manifest and returns the aggregate score.
func (e *Evaluator) Evaluate(ctx context.Context, in Input) model.Evaluation {
	ev := model.Evaluation{
		Status:    "fail",
		Points:    0,
		MaxPoints: in.Manifest.maxPoints(),
		Checks:    nil,
		Breakdown: model.Breakdown{},
	}

	axes := []struct {
		name  string
		ptr   *int
		check []Check
	}{
		{"correctness", &ev.Breakdown.Correctness, in.Manifest.Rubric.Correctness},
		{"scope", &ev.Breakdown.Scope, in.Manifest.Rubric.Scope},
		{"pattern", &ev.Breakdown.Pattern, in.Manifest.Rubric.Pattern},
		{"verification", &ev.Breakdown.Verification, in.Manifest.Rubric.Verification},
		{"cleanup", &ev.Breakdown.Cleanup, in.Manifest.Rubric.Cleanup},
	}

	for _, axis := range axes {
		passed, max := 0, 0
		for _, c := range axis.check {
			res := e.runCheck(ctx, in, c)
			if res.Pass {
				passed += c.Weight
			}
			max += c.Weight
			ev.Checks = append(ev.Checks, res)
		}
		*axis.ptr = passed
		ev.Points += passed
		_ = max
	}

	if ev.MaxPoints > 0 {
		switch {
		case ev.Points >= ev.MaxPoints:
			ev.Status = "pass"
		case ev.Points > 0:
			ev.Status = "partial"
		default:
			ev.Status = "fail"
		}
	} else {
		ev.Status = "pass"
	}

	if ev.Status == "pass" {
		ev.Summary = in.Manifest.Labels.Pass
	} else if ev.Status == "partial" {
		ev.Summary = in.Manifest.Labels.Partial
	} else {
		ev.Summary = in.Manifest.Labels.Fail
	}
	if ev.Summary == "" {
		ev.Summary = ev.Status
	}

	return ev
}

// runCheck dispatches a single check based on its type.
func (e *Evaluator) runCheck(ctx context.Context, in Input, check Check) model.CheckResult {
	result := model.CheckResult{Name: check.Name, Pass: false, Weight: check.Weight}
	if check.Weight == 0 {
		result.Pass = true
		return result
	}

	switch check.Type {
	case "file_contains":
		result.Pass, result.Detail = runFileContains(in, check.Params, false)
	case "file_not_contains":
		result.Pass, result.Detail = runFileContains(in, check.Params, true)
	case "files_changed_only":
		result.Pass, result.Detail = runFilesChangedOnly(in, check.Params)
	case "behavioral_test":
		result.Pass, result.Detail = runBehavioralTest(ctx, in, check.Params)
	case "command":
		result.Pass, result.Detail = runCommandCheck(ctx, in, check.Params)
	case "trace_read_before_edit":
		result.Pass, result.Detail = runTraceReadBeforeEdit(in, check.Params)
	default:
		result.Detail = fmt.Sprintf("unsupported check type %q", check.Type)
	}

	return result
}

func runFileContains(in Input, params map[string]any, invert bool) (bool, string) {
	file := stringParam(params, "file")
	pattern := stringParam(params, "pattern")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if pattern == "" {
		return false, "missing 'pattern' parameter"
	}

	p := filepath.Join(in.WorkDir, file)
	data, err := os.ReadFile(p)
	if err != nil {
		if invert {
			return true, fmt.Sprintf("file %s does not exist", file)
		}
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	content := string(data)

	matched, err := matchPattern(pattern, content)
	if err != nil {
		return false, fmt.Sprintf("invalid pattern %q: %v", pattern, err)
	}

	if invert {
		if matched {
			return false, fmt.Sprintf("file %s unexpectedly matched pattern %q", file, pattern)
		}
		return true, fmt.Sprintf("file %s does not match pattern %q", file, pattern)
	}
	if !matched {
		return false, fmt.Sprintf("file %s did not match pattern %q", file, pattern)
	}
	return true, fmt.Sprintf("file %s matches pattern %q", file, pattern)
}

func runFilesChangedOnly(in Input, params map[string]any) (bool, string) {
	allowedList := stringSliceParam(params, "allowed")
	allowed := make(map[string]struct{}, len(allowedList))
	for _, a := range allowedList {
		allowed[a] = struct{}{}
	}

	changed, deleted, err := diffWorkspace(in)
	if err != nil {
		return false, fmt.Sprintf("workspace diff error: %v", err)
	}

	var unexpected []string
	for _, p := range changed {
		if _, ok := allowed[p]; !ok {
			unexpected = append(unexpected, p)
		}
	}
	for _, p := range deleted {
		if _, ok := allowed[p]; !ok {
			unexpected = append(unexpected, p+" (deleted)")
		}
	}

	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		return false, fmt.Sprintf("unexpected changes: %s", strings.Join(unexpected, ", "))
	}
	return true, "only allowed paths changed"
}

func runBehavioralTest(ctx context.Context, in Input, params map[string]any) (bool, string) {
	command := stringParam(params, "runner")
	if command == "" {
		return false, "missing 'runner' parameter"
	}

	tmp, err := os.MkdirTemp("", "sb-behavior-")
	if err != nil {
		return false, fmt.Sprintf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	files := stringSliceParam(params, "files")
	for _, f := range files {
		src := filepath.Join(in.WorkDir, f)
		dst := filepath.Join(tmp, f)
		if err := copyFile(src, dst); err != nil {
			return false, fmt.Sprintf("copy %s: %v", f, err)
		}
	}

	for _, hf := range in.Manifest.HiddenFixtures {
		if hf.Src == "" || hf.Dest == "" {
			continue
		}
		src := filepath.Join(in.Dir, hf.Src)
		dst := filepath.Join(tmp, hf.Dest)
		if err := copyFile(src, dst); err != nil {
			return false, fmt.Sprintf("copy hidden fixture %s: %v", hf.Src, err)
		}
	}

	timeout := time.Duration(intParam(params, "timeout_ms", 30000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ok, output, err := runShellCommand(ctx, tmp, command, timeout)
	if err != nil {
		return false, fmt.Sprintf("behavioral test failed: %v", err)
	}
	if !ok {
		return false, fmt.Sprintf("behavioral test command exited non-zero: %s", output)
	}
	return true, fmt.Sprintf("behavioral test passed: %s", output)
}

func runCommandCheck(ctx context.Context, in Input, params map[string]any) (bool, string) {
	command := stringParam(params, "command")
	if command == "" {
		return false, "missing 'command' parameter"
	}
	timeout := time.Duration(intParam(params, "timeout_ms", 30000)) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ok, output, err := runShellCommand(ctx, in.WorkDir, command, timeout)
	if err != nil {
		return false, fmt.Sprintf("command error: %v", err)
	}
	if !ok {
		return false, fmt.Sprintf("command exited non-zero: %s", output)
	}
	return true, output
}

func runTraceReadBeforeEdit(in Input, params map[string]any) (bool, string) {
	target := stringParam(params, "path")
	if target == "" {
		return false, "missing 'path' parameter"
	}

	var editIndex = -1
	var readBefore bool
	for i, tc := range in.ToolCalls {
		path := toolCallPath(tc)
		if path == "" {
			continue
		}

		switch tc.Name {
		case "write", "edit":
			if path == target && editIndex == -1 {
				editIndex = i
			}
		case "read":
			if path == target && (editIndex == -1 || i < editIndex) {
				readBefore = true
			}
		}
	}

	if editIndex == -1 {
		return true, "no edit/write of the target path was recorded"
	}
	if readBefore {
		return true, "target path was read before first edit/write"
	}
	return false, "target path was not read before first edit/write"
}

// diffWorkspace returns the changed and deleted paths relative to the workspace root.
func diffWorkspace(in Input) (changed []string, deleted []string, err error) {
	workspaceDir := in.WorkDir
	if in.Manifest.Workspace.Root != "" {
		workspaceDir = filepath.Join(workspaceDir, in.Manifest.Workspace.Root)
	}

	current, err := walkFiles(workspaceDir)
	if err != nil {
		return nil, nil, err
	}

	pristine, err := walkFiles(in.PristineDir)
	if err != nil {
		pristine = map[string]string{}
	}

	rootName := in.Manifest.Workspace.Root
	for rel, content := range current {
		p, exists := pristine[rel]
		if !exists || p != content {
			changed = append(changed, filepath.ToSlash(filepath.Join(rootName, rel)))
		}
	}
	for rel := range pristine {
		if _, exists := current[rel]; !exists {
			deleted = append(deleted, filepath.ToSlash(filepath.Join(rootName, rel)))
		}
	}

	sort.Strings(changed)
	sort.Strings(deleted)
	return changed, deleted, nil
}

func matchPattern(pattern, content string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err == nil {
		return re.MatchString(content), nil
	}
	return strings.Contains(content, pattern), nil
}

func runShellCommand(ctx context.Context, cwd, command string, timeout time.Duration) (bool, string, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var shell, arg string
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		arg = "/c"
	} else {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		arg = "-lc"
	}

	cmd := exec.CommandContext(ctx, shell, arg, command)
	cmd.Dir = cwd
	cmd.Env = shellEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if out == "" {
		out = strings.TrimSpace(stderr.String())
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, out, fmt.Errorf("timed out")
		}
		return false, out, err
	}
	return true, out, nil
}

func shellEnv() []string {
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

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func toolCallPath(tc model.ToolCall) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Args), &args); err != nil {
		return ""
	}
	v, ok := args["path"].(string)
	if !ok {
		return ""
	}
	return v
}

func stringParam(params map[string]any, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func stringSliceParam(params map[string]any, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func intParam(params map[string]any, key string, defaultVal int) int {
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func (m Manifest) maxPoints() int {
	total := 0
	lists := [][]Check{
		m.Rubric.Correctness,
		m.Rubric.Scope,
		m.Rubric.Pattern,
		m.Rubric.Verification,
		m.Rubric.Cleanup,
	}
	for _, list := range lists {
		for _, c := range list {
			total += c.Weight
		}
	}
	return total
}
