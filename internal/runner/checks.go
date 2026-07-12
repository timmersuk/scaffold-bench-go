package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// runFunctionEquals compares two functions extracted from the same file.
// When invert is true, the check requires the functions to differ.
func runFunctionEquals(in Input, params map[string]any, invert bool) (bool, string) {
	file := stringParam(params, "file")
	functionA := stringParam(params, "functionA")
	functionB := stringParam(params, "functionB")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if functionA == "" || functionB == "" {
		return false, "missing 'functionA' or 'functionB' parameter"
	}

	content, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}

	a := extractFunction(string(content), functionA)
	b := extractFunction(string(content), functionB)
	if a == "" || b == "" {
		return false, fmt.Sprintf("could not extract %s or %s from %s", functionA, functionB, file)
	}

	match := a == b
	if invert {
		if !match {
			return true, fmt.Sprintf("%s and %s differ in %s", functionA, functionB, file)
		}
		return false, fmt.Sprintf("%s and %s unexpectedly match in %s", functionA, functionB, file)
	}
	if match {
		return true, fmt.Sprintf("%s and %s match in %s", functionA, functionB, file)
	}
	return false, fmt.Sprintf("%s and %s do not match in %s", functionA, functionB, file)
}

// runFunctionEqualsOriginal compares a function in the mutated workspace to
// its copy in the pristine directory.
func runFunctionEqualsOriginal(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	functionName := stringParam(params, "function")
	if file == "" {
		return false, "missing 'file' parameter"
	}
	if functionName == "" {
		return false, "missing 'function' parameter"
	}

	currentPath := filepath.Join(in.WorkDir, file)
	originalPath := resolvePristinePath(in, file)

	current, err := os.ReadFile(currentPath)
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	original, err := os.ReadFile(originalPath)
	if err != nil {
		return false, fmt.Sprintf("could not read pristine %s: %v", file, err)
	}

	currentFn := extractFunction(string(current), functionName)
	originalFn := extractFunction(string(original), functionName)
	if currentFn == "" || originalFn == "" {
		return false, fmt.Sprintf("could not extract %s from %s or its pristine copy", functionName, file)
	}

	if currentFn == originalFn {
		return true, fmt.Sprintf("%s is unchanged in %s", functionName, file)
	}
	return false, fmt.Sprintf("%s changed in %s", functionName, file)
}

// runNoFilesChanged requires the workspace to be identical to the pristine copy.
func runNoFilesChanged(in Input, params map[string]any) (bool, string) {
	_ = params
	changed, deleted, err := diffWorkspace(in)
	if err != nil {
		return false, fmt.Sprintf("workspace diff error: %v", err)
	}
	if len(changed) == 0 && len(deleted) == 0 {
		return true, "no files changed"
	}
	var parts []string
	if len(changed) > 0 {
		parts = append(parts, "changed: "+strings.Join(changed, ", "))
	}
	if len(deleted) > 0 {
		parts = append(parts, "deleted: "+strings.Join(deleted, ", "))
	}
	return false, strings.Join(parts, "; ")
}

// runTraceSearchBeforeEdit requires that the first mutating tool call for a
// path is preceded by a read, ls, or bash referencing that path.
func runTraceSearchBeforeEdit(in Input, params map[string]any) (bool, string) {
	target := stringParam(params, "path")
	if target == "" {
		return false, "missing 'path' parameter"
	}

	mutationIdx := -1
	for i, tc := range in.ToolCalls {
		if (tc.Name == "edit" || tc.Name == "write") && toolCallPath(tc) == target {
			mutationIdx = i
			break
		}
	}
	if mutationIdx == -1 {
		return true, "no edit/write of the target path was recorded"
	}

	for i := 0; i < mutationIdx; i++ {
		if isSearchLikeTool(in.ToolCalls[i]) && toolCallReferencesPath(in.ToolCalls[i], target) {
			return true, "target path was searched/read before first edit/write"
		}
	}
	return false, "target path was not searched before first edit/write"
}

// runTraceVerificationAfterChange requires that a verification command or bash
// call passed after the first mutating change.
func runTraceVerificationAfterChange(in Input, params map[string]any) (bool, string) {
	_ = params
	mutationIdx := -1
	for i, tc := range in.ToolCalls {
		if tc.Name == "edit" || tc.Name == "write" {
			mutationIdx = i
			break
		}
	}
	if mutationIdx == -1 {
		return false, "no mutating change was recorded"
	}

	for i := mutationIdx + 1; i < len(in.ToolCalls); i++ {
		tc := in.ToolCalls[i]
		if tc.Name != "bash" {
			continue
		}
		cmd := bashCommandFromToolCall(tc)
		if isVerifyCommand(cmd) && bashPassed(tc) {
			return true, fmt.Sprintf("verification command passed after first change: %s", cmd)
		}
	}
	return false, "no passing verification command found after first change"
}

// runNoAddedComments compares the comments in the file to its pristine copy
// and requires that no new comments were introduced.
func runNoAddedComments(in Input, params map[string]any) (bool, string) {
	file := stringParam(params, "file")
	if file == "" {
		return false, "missing 'file' parameter"
	}

	current, err := os.ReadFile(filepath.Join(in.WorkDir, file))
	if err != nil {
		return false, fmt.Sprintf("could not read %s: %v", file, err)
	}
	original, err := os.ReadFile(resolvePristinePath(in, file))
	if err != nil {
		return false, fmt.Sprintf("could not read pristine %s: %v", file, err)
	}

	currentComments := extractComments(string(current))
	originalComments := extractComments(string(original))
	if len(currentComments) != len(originalComments) {
		return false, fmt.Sprintf("comment count changed: %d -> %d", len(originalComments), len(currentComments))
	}
	for i := range currentComments {
		if currentComments[i] != originalComments[i] {
			return false, "comment set differs from pristine"
		}
	}
	return true, "no new comments"
}

// extractFunction returns the brace-aware text of a JS/TS-like function.
// It supports `function name(...) { ... }` and `const name = (...) => { ... }`.
func extractFunction(source, name string) string {
	quoted := regexp.QuoteMeta(name)
	headers := []*regexp.Regexp{
		regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+` + quoted + `\s*\([\s\S]*?\)\s*\{`),
		regexp.MustCompile(`(?:export\s+)?(?:const|let|var)\s+` + quoted + `\s*=\s*(?:async\s*)?\([\s\S]*?\)\s*=>\s*\{`),
		regexp.MustCompile(`(?:export\s+)?(?:const|let|var)\s+` + quoted + `\s*=\s*(?:async\s*)?[A-Za-z_$][\w$]*\s*=>\s*\{`),
	}
	for _, re := range headers {
		if loc := re.FindStringIndex(source); loc != nil {
			return extractBracedBlock(source, loc[0])
		}
	}
	return ""
}

func extractBracedBlock(source string, start int) string {
	depth := 0
	started := false
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
			started = true
		case '}':
			depth--
			if started && depth == 0 {
				return source[start : i+1]
			}
		}
	}
	return ""
}

// extractComments returns trimmed JS/TS comments, sorted for stable comparison.
func extractComments(source string) []string {
	re := regexp.MustCompile(`//[^\n]*|/\*[\s\S]*?\*/`)
	matches := re.FindAllString(source, -1)
	for i, m := range matches {
		matches[i] = strings.TrimSpace(m)
	}
	sort.Strings(matches)
	return matches
}

// resolvePristinePath maps a workspace-relative file path to the pristine root.
func resolvePristinePath(in Input, file string) string {
	rel := file
	if root := in.Manifest.Workspace.Root; root != "" {
		prefix := root + "/"
		if strings.HasPrefix(file, prefix) {
			rel = file[len(prefix):]
		}
	}
	return filepath.Join(in.PristineDir, rel)
}

func isSearchLikeTool(tc model.ToolCall) bool {
	switch tc.Name {
	case "read", "ls", "bash":
		return true
	}
	return false
}

func toolCallReferencesPath(tc model.ToolCall, target string) bool {
	switch tc.Name {
	case "read", "ls":
		return toolCallPath(tc) == target
	case "bash":
		return strings.Contains(bashCommandFromToolCall(tc), target)
	}
	return false
}

func bashCommandFromToolCall(tc model.ToolCall) string {
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Args), &args); err == nil {
		if c, ok := args["command"].(string); ok {
			return c
		}
	}
	return tc.Args
}

func bashPassed(tc model.ToolCall) bool {
	if tc.Name != "bash" {
		return false
	}
	re := regexp.MustCompile(`(?m)^exit_code:\s*(\d+)`)
	m := re.FindStringSubmatch(tc.Result)
	if len(m) < 2 {
		return false
	}
	return m[1] == "0"
}

// isVerifyCommand decides whether a bash command counts as self-verification.
func isVerifyCommand(command string) bool {
	if verifyRunnerPattern.MatchString(command) {
		return true
	}
	return verifyGenericPattern.MatchString(command) && !verifyInspectionLead.MatchString(command)
}

var (
	verifyRunnerPattern  = regexp.MustCompile(`\b(?:bun\s+test|npm\s+(?:run\s+)?test|npx\s+(?:vitest|jest|tsc)\b|node\s+--test|node\s+-c\b|vitest\b|jest\b|pytest\b|cargo\s+(?:test|check)\b|go\s+(?:test|vet|build)\b|php\s+-l\b|shellcheck\b|tsc\b|make\s+(?:test|check)\b|deno\s+test\b|(?:node|bun|deno|tsx|python3?)\s+\S*(?:test|spec)\S*\.\w+)`)
	verifyGenericPattern = regexp.MustCompile(`\b(?:test|spec)s?\b`)
	verifyInspectionLead = regexp.MustCompile(`^\s*(?:sudo\s+)?(?:cat|bat|less|more|head|tail|ls|ll|tree|grep|rg|ag|find|fd|sed|awk|gawk|rm|cp|mv|touch|mkdir|echo|printf|stat|file|wc|du|nano|vim|nvim|git|diff|chmod|chown|cd|export|open|xdg-open)\b`)
)
