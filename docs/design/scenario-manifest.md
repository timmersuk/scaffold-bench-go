# Scenario manifest schema and evaluator interface

This document records the design decision for [#9](https://github.com/timmersuk/scaffold-bench-go/issues/9).

## Goal

Replace the compiled TypeScript scenario modules with neutral, hand-editable manifest files and a Go evaluator engine. A manifest must be able to express all existing scaffold-bench scenarios, including the 10-point rubric, scope checks, behavioral tests, tool-trace checks, and pattern checks.

## Manifest file layout

One manifest per scenario, e.g. `scenarios/SB-01/manifest.yaml`:

```yaml
id: "SB-01"
name: "fix-throttle"
category: "surgical-edit"
family: "regex-style"
difficulty: "low"
rubricKind: "10pt"
signalType: "behavioral"
track: "execution"
prompt: |
  The throttle function in playground/utils.js is broken...

workspace:
  root: "playground"
  pristineDir: "_fixtures/playground/SB-01"
  allowedPaths:
    - "playground/utils.js"

requires: ["bun"]

setup:
  files:
    - src: "_fixtures/playground/SB-48/items_test.go"
      dest: "playground/go-api/items_test.go"

hiddenFixtures:
  - src: "_fixtures/behaviors/SB-01/throttle.behavior.test.mjs"
    dest: "playground/utils.behavior.test.mjs"

build:
  commands:
    - "cd {{.WorkDir}}/playground && bun install"

rubric:
  correctness:
    - name: "throttle differs from debounce"
      type: "function_not_equal"
      weight: 1
      params:
        file: "playground/utils.js"
        functionA: "throttle"
        functionB: "debounce"

    - name: "throttle actually throttles"
      type: "behavioral_test"
      weight: 2
      params:
        runner: "bun-test"
        testFile: "playground/utils.behavior.test.mjs"
        files: ["playground/utils.js"]

  scope:
    - name: "edited only utils.js"
      type: "files_changed_only"
      weight: 2
      params:
        allowed:
          - "playground/utils.js"

  pattern:
    - name: "debounce unchanged"
      type: "function_equals_original"
      weight: 1
      params:
        file: "playground/utils.js"
        function: "debounce"

    - name: "formatDate unchanged"
      type: "function_equals_original"
      weight: 1
      params:
        file: "playground/utils.js"
        function: "formatDate"

  verification:
    - name: "read before edit"
      type: "trace_read_before_edit"
      weight: 1
      params:
        path: "playground/utils.js"

  cleanup:
    - name: "no added comments"
      type: "no_added_comments"
      weight: 1
      params:
        file: "playground/utils.js"

    - name: "no console.log"
      type: "file_not_contains"
      weight: 1
      params:
        file: "playground/utils.js"
        pattern: "console\\.log\\s*\\("

labels:
  pass: "Fixed throttle with real logic, left adjacent code untouched."
  partial: "Fixed throttle but also touched adjacent code, or weak throttle logic."
  fail: "Did not produce a valid throttle fix."
```

## Schema (YAML/JSON)

```yaml
Meta:
  id: string
  name: string
  category: string
  family: string
  difficulty: "low" | "medium" | "high"
  rubricKind: "10pt" | "custom-5pt" | "custom-3pt"
  signalType: "behavioral" | "regex-shape" | "stdout" | "trace" | "latency"
  track?: "execution" | "problem-solving"
  prompt: string

Workspace:
  root: string
  pristineDir?: string          # source of truth for diff/scope checks
  allowedPaths?: string[]       # default allowed set for scope checks

FileMapping:
  src: string                   # path relative to scenario directory
  dest: string                  # path relative to workspace root

Build:
  commands?: string[]           # shell commands run before checks
  env?: map<string, string>

RubricCheck:
  name: string
  type: string
  weight: number
  params: map<string, any>
  onSkip?: "pass" | "fail" | "ignore"

Rubric:
  correctness: RubricCheck[]
  scope: RubricCheck[]
  pattern: RubricCheck[]
  verification: RubricCheck[]
  cleanup: RubricCheck[]

Labels:
  pass: string
  partial: string
  fail: string

Manifest:
  meta: Meta
  workspace: Workspace
  requires?: string[]
  setup?: { files?: FileMapping[] }
  hiddenFixtures?: FileMapping[]
  build?: Build
  rubric: Rubric
  labels: Labels
```

## Check type registry

| Type | Purpose |
|------|---------|
| `file_contains` / `file_not_contains` | Regex over file content |
| `function_equals` / `function_not_equal` | Brace-aware function extraction and comparison |
| `function_equals_original` | Compare a function to its pristine copy |
| `files_changed_only` | Diff workspace vs pristine; only listed paths may change |
| `no_files_changed` | Diff workspace; require zero changes |
| `behavioral_test` | Copy named files + hidden fixture to a temp dir and run a test runner |
| `command` | Run arbitrary external command; pass if exit code is 0 |
| `trace_read_before_edit` | Tool-call trace shows a `read` of the path before first edit/write |
| `trace_search_before_edit` | Tool-call trace shows a search/grep/ls of the path before first edit/write |
| `trace_verification_after_change` | A verification command passed after the first mutating change |
| `no_added_comments` | Compare comments with pristine copy |
| `no_extra_functions` | Count top-level function declarations vs pristine |
| `ast_*` | Native Go checks that inspect TypeScript/TSX source for the specific patterns used by a scenario |

Test runners for `behavioral_test`: `bun-test`, `go-test`, `cargo-test`, `pytest`, `php`, `shellcheck`, or a literal command template.

## Go evaluator interface

```go
package evaluator

import "context"

// Manifest is the parsed scenario definition.
type Manifest struct {
    Meta         Meta       `yaml:"meta"`
    Workspace    Workspace  `yaml:"workspace"`
    Requires     []string   `yaml:"requires"`
    Setup        *Setup     `yaml:"setup,omitempty"`
    HiddenFixtures []Fixture  `yaml:"hiddenFixtures,omitempty"`
    Build        *Build     `yaml:"build,omitempty"`
    Rubric       Rubric     `yaml:"rubric"`
    Labels       Labels     `yaml:"labels"`
}

type Meta struct {
    ID         string `yaml:"id"`
    Name       string `yaml:"name"`
    Category   string `yaml:"category"`
    Family     string `yaml:"family"`
    Difficulty string `yaml:"difficulty"`
    RubricKind string `yaml:"rubricKind"`
    SignalType string `yaml:"signalType"`
    Track      string `yaml:"track,omitempty"`
    Prompt     string `yaml:"prompt"`
}

type Workspace struct {
    Root         string   `yaml:"root"`
    PristineDir  string   `yaml:"pristineDir,omitempty"`
    AllowedPaths []string `yaml:"allowedPaths,omitempty"`
}

type Setup struct {
    Files []FileMapping `yaml:"files,omitempty"`
}

type FileMapping struct {
    Src  string `yaml:"src"`
    Dest string `yaml:"dest"`
}

type Build struct {
    Commands []string          `yaml:"commands,omitempty"`
    Env      map[string]string `yaml:"env,omitempty"`
}

type Rubric struct {
    Correctness []Check `yaml:"correctness"`
    Scope       []Check `yaml:"scope"`
    Pattern     []Check `yaml:"pattern"`
    Verification []Check `yaml:"verification"`
    Cleanup     []Check `yaml:"cleanup"`
}

type Check struct {
    Name    string         `yaml:"name"`
    Type    string         `yaml:"type"`
    Weight  int            `yaml:"weight"`
    Params  map[string]any `yaml:"params"`
    OnSkip  string         `yaml:"onSkip,omitempty"`
}

type Labels struct {
    Pass    string `yaml:"pass"`
    Partial string `yaml:"partial"`
    Fail    string `yaml:"fail"`
}

// Input is everything the evaluator needs to score a run.
type Input struct {
    Manifest    Manifest
    WorkDir     string      // path to the mutated workspace
    PristineDir string      // path to the pristine copy
    ToolCalls   []ToolCall  // full tool-call trace
}

type ToolCall struct {
    Name      string `json:"name"`
    Arguments any    `json:"arguments"`
    Result    string `json:"result"`
}

// Evaluation is the scoring result.
type Evaluation struct {
    Status    string           `json:"status"`
    Points    int              `json:"points"`
    MaxPoints int              `json:"maxPoints"`
    Breakdown Breakdown        `json:"breakdown"`
    Checks    []CheckResult    `json:"checks"`
    ErrorKind string           `json:"errorKind,omitempty"`
    Error     string           `json:"error,omitempty"`
}

type Breakdown struct {
    Correctness  int `json:"correctness"`
    Scope        int `json:"scope"`
    Pattern      int `json:"pattern"`
    Verification int `json:"verification"`
    Cleanup      int `json:"cleanup"`
}

type CheckResult struct {
    Name    string `json:"name"`
    Passed  bool   `json:"passed"`
    Weight  int    `json:"weight"`
    Detail  string `json:"detail,omitempty"`
}

// Engine evaluates manifests.
type Engine interface {
    Evaluate(ctx context.Context, in Input) (Evaluation, error)
}
```

## Migration strategy

1. Freeze TS behavior for a representative subset (SB-01, SB-10, SB-14, SB-48, one AST-heavy scenario).
2. Implement the manifest loader, the check registry, and the rubric scorer in Go.
3. Translate the representative subset to manifests and run them against golden workspaces to verify scores match.
4. Port the remaining scenarios in batches:
   - Regex/scope/behavioral scenarios are mostly declarative.
   - AST-dependent scenarios get an `ast_*` check type implemented as a narrow native Go check. The check is validated against the upstream golden fixtures for that scenario, not against every possible TS edge case.
   - Highly custom scenarios keep a tiny Go `Evaluator` plugin registered by scenario ID.
5. Keep external test runners (`bun test`, `go test`, etc.) as subprocess invocations; only the orchestration and scoring move to Go.

## Risks

- **AST fidelity**: `evaluators/ast.ts` uses the TypeScript compiler API. The neutral manifest can express *what* to check; native Go checks target the exact shapes used by ported scenarios and are validated against upstream gate fixtures.
- **Tool-trace fidelity**: Some checks depend on the exact output strings of `read`/`edit`/`write`/`ls`/`bash`. The Go tool handlers must preserve these strings.
- **Hidden tests**: Behavioral tests must remain outside the model workspace. The manifest `hiddenFixtures` plus a temp-copy runner handles this.
