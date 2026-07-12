package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

func makeManifest(rubric Rubric) Manifest {
	return Manifest{
		Workspace: Workspace{Root: "playground"},
		Rubric:    rubric,
		Labels: Labels{
			Pass:    "pass",
			Partial: "partial",
			Fail:    "fail",
		},
	}
}

func TestEvaluatorFileContains(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644)

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "contains hello", Type: "file_contains", Weight: 8, Params: map[string]any{
				"file": "playground/hello.txt", "pattern": "^hello$",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 8 {
		t.Fatalf("expected 8 points, got %d", res.Points)
	}
	if res.Status != "pass" {
		t.Fatalf("expected pass, got %s", res.Status)
	}
}

func TestEvaluatorFileNotContains(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("goodbye"), 0o644)

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "not bad", Type: "file_not_contains", Weight: 5, Params: map[string]any{
				"file": "playground/hello.txt", "pattern": "bad",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 5 {
		t.Fatalf("expected 5 points, got %d", res.Points)
	}
}

func TestEvaluatorFilesChangedOnly(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644)

	m := makeManifest(Rubric{
		Scope: []Check{
			{Name: "only hello", Type: "files_changed_only", Weight: 2, Params: map[string]any{
				"allowed": []any{"playground/hello.txt"},
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 2 {
		t.Fatalf("expected 2 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorFilesChangedOnlyFails(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(root, "extra.txt"), []byte("extra"), 0o644)

	m := makeManifest(Rubric{
		Scope: []Check{
			{Name: "only hello", Type: "files_changed_only", Weight: 2, Params: map[string]any{
				"allowed": []any{"playground/hello.txt"},
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 0 {
		t.Fatalf("expected 0 points, got %d", res.Points)
	}
}

func TestEvaluatorCommand(t *testing.T) {
	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "echo", Type: "command", Weight: 4, Params: map[string]any{
				"command": "echo ok",
			}},
		},
	})
	tmp := t.TempDir()
	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 4 {
		t.Fatalf("expected 4 points, got %d: %#v", res.Points, res)
	}
}

func TestEvaluatorTraceReadBeforeEdit(t *testing.T) {
	m := makeManifest(Rubric{
		Verification: []Check{
			{Name: "read before edit", Type: "trace_read_before_edit", Weight: 3, Params: map[string]any{
				"path": "playground/hello.txt",
			}},
		},
	})
	calls := []model.ToolCall{
		{Name: "read", Args: `{"path":"playground/hello.txt"}`},
		{Name: "write", Args: `{"path":"playground/hello.txt","content":"hello"}`},
	}
	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir(), ToolCalls: calls})
	if res.Points != 3 {
		t.Fatalf("expected 3 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorBehavioralTest(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0o644)

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "behavior", Type: "behavioral_test", Weight: 6, Params: map[string]any{
				"runner": "exit 0",
				"files":  []any{"playground/hello.txt"},
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 6 {
		t.Fatalf("expected 6 points, got %d: %+v; output: %s", res.Points, res.Checks, res.Summary)
	}
}

func TestEvaluatorBehavioralTestHiddenFixture(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir playground: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "utils.js"), []byte("function hello() { return 'hi'; }"), 0o644); err != nil {
		t.Fatalf("write utils.js: %v", err)
	}

	hiddenDir := filepath.Join(tmp, "hidden")
	if err := os.MkdirAll(filepath.Join(hiddenDir, "playground"), 0o755); err != nil {
		t.Fatalf("mkdir hidden fixture dir: %v", err)
	}
	script := "exit 0"
	if err := os.WriteFile(filepath.Join(hiddenDir, "playground", "utils.behavior.test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatalf("write hidden fixture: %v", err)
	}

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "behavior", Type: "behavioral_test", Weight: 6, Params: map[string]any{
				"runner":   "shell",
				"testFile": "playground/utils.behavior.test.mjs",
				"files":    []any{"playground/utils.js"},
			}},
		},
	})
	m.HiddenFixtures = []FileMapping{
		{Src: "_fixtures/behaviors/SB-01/throttle.behavior.test.mjs", Dest: "playground/utils.behavior.test.mjs"},
	}

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, HiddenDir: hiddenDir})
	if res.Points != 6 {
		t.Fatalf("expected 6 points, got %d: %+v", res.Points, res.Checks)
	}
}

func writePristine(t *testing.T, base string, rel string, content []byte) string {
	t.Helper()
	dir := filepath.Join(base, filepath.Dir(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir pristine dir: %v", err)
	}
	path := filepath.Join(base, rel)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write pristine file: %v", err)
	}
	return path
}

func TestEvaluatorFunctionEquals(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte(`
function throttle(fn, wait) { return fn; }
function debounce(fn, wait) { return fn; }
const format = (x) => { return x; }
`), 0o644)

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "throttle equals throttle", Type: "function_equals", Weight: 1, Params: map[string]any{
				"file": "playground/utils.js", "functionA": "throttle", "functionB": "throttle",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorFunctionNotEqual(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte(`
function throttle(fn, wait) { return fn; }
const format = (x) => { return x; }
`), 0o644)

	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "throttle differs from format", Type: "function_not_equal", Weight: 1, Params: map[string]any{
				"file": "playground/utils.js", "functionA": "throttle", "functionB": "format",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorFunctionEqualsOriginal(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte(`function debounce(fn, wait) { return fn; }`), 0o644)

	pristine := filepath.Join(tmp, "pristine")
	writePristine(t, pristine, "utils.js", []byte(`function debounce(fn, wait) { return fn; }`))

	m := makeManifest(Rubric{
		Pattern: []Check{
			{Name: "debounce unchanged", Type: "function_equals_original", Weight: 1, Params: map[string]any{
				"file": "playground/utils.js", "function": "debounce",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, PristineDir: pristine})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorNoFilesChanged(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte("function a() {}"), 0o644)

	pristine := filepath.Join(tmp, "pristine")
	writePristine(t, pristine, "utils.js", []byte("function a() {}"))

	m := makeManifest(Rubric{
		Scope: []Check{
			{Name: "nothing changed", Type: "no_files_changed", Weight: 2, Params: map[string]any{}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, PristineDir: pristine})
	if res.Points != 2 {
		t.Fatalf("expected 2 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorNoFilesChangedFails(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte("function a() { return 1; }"), 0o644)

	pristine := filepath.Join(tmp, "pristine")
	writePristine(t, pristine, "utils.js", []byte("function a() {}"))

	m := makeManifest(Rubric{
		Scope: []Check{
			{Name: "nothing changed", Type: "no_files_changed", Weight: 2, Params: map[string]any{}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, PristineDir: pristine})
	if res.Points != 0 {
		t.Fatalf("expected 0 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorTraceSearchBeforeEdit(t *testing.T) {
	m := makeManifest(Rubric{
		Verification: []Check{
			{Name: "search before edit", Type: "trace_search_before_edit", Weight: 1, Params: map[string]any{
				"path": "playground/utils.js",
			}},
		},
	})
	calls := []model.ToolCall{
		{Name: "ls", Args: `{"path":"playground/utils.js"}`},
		{Name: "write", Args: `{"path":"playground/utils.js","content":"x"}`},
	}

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir(), ToolCalls: calls})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorTraceSearchBeforeEditFails(t *testing.T) {
	m := makeManifest(Rubric{
		Verification: []Check{
			{Name: "search before edit", Type: "trace_search_before_edit", Weight: 1, Params: map[string]any{
				"path": "playground/utils.js",
			}},
		},
	})
	calls := []model.ToolCall{
		{Name: "write", Args: `{"path":"playground/utils.js","content":"x"}`},
		{Name: "ls", Args: `{"path":"playground/utils.js"}`},
	}

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir(), ToolCalls: calls})
	if res.Points != 0 {
		t.Fatalf("expected 0 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorTraceVerificationAfterChange(t *testing.T) {
	m := makeManifest(Rubric{
		Verification: []Check{
			{Name: "verify after change", Type: "trace_verification_after_change", Weight: 1, Params: map[string]any{}},
		},
	})
	calls := []model.ToolCall{
		{Name: "edit", Args: `{"path":"playground/utils.js","old_str":"a","new_str":"b"}`, Result: "exit_code: 0"},
		{Name: "bash", Args: `{"command":"go test ./..."}`, Result: "exit_code: 0\n\nstdout:\nok"},
	}

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir(), ToolCalls: calls})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorTraceVerificationAfterChangeFailsWithoutVerify(t *testing.T) {
	m := makeManifest(Rubric{
		Verification: []Check{
			{Name: "verify after change", Type: "trace_verification_after_change", Weight: 1, Params: map[string]any{}},
		},
	})
	calls := []model.ToolCall{
		{Name: "edit", Args: `{"path":"playground/utils.js","old_str":"a","new_str":"b"}`, Result: "exit_code: 0"},
		{Name: "bash", Args: `{"command":"cat playground/utils.js"}`, Result: "exit_code: 0"},
	}

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir(), ToolCalls: calls})
	if res.Points != 0 {
		t.Fatalf("expected 0 points, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorNoAddedComments(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte("// keep\nfunction a() {}\n/* end */"), 0o644)

	pristine := filepath.Join(tmp, "pristine")
	writePristine(t, pristine, "utils.js", []byte("// keep\nfunction a() {}\n/* end */"))

	m := makeManifest(Rubric{
		Cleanup: []Check{
			{Name: "no added comments", Type: "no_added_comments", Weight: 1, Params: map[string]any{
				"file": "playground/utils.js",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, PristineDir: pristine})
	if res.Points != 1 {
		t.Fatalf("expected 1 point, got %d: %v", res.Points, res.Checks)
	}
}

func TestEvaluatorNoAddedCommentsFails(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "utils.js"), []byte("// keep\nfunction a() {}\n/* end */\n// new"), 0o644)

	pristine := filepath.Join(tmp, "pristine")
	writePristine(t, pristine, "utils.js", []byte("// keep\nfunction a() {}\n/* end */"))

	m := makeManifest(Rubric{
		Cleanup: []Check{
			{Name: "no added comments", Type: "no_added_comments", Weight: 1, Params: map[string]any{
				"file": "playground/utils.js",
			}},
		},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp, PristineDir: pristine})
	if res.Points != 0 {
		t.Fatalf("expected 0 points, got %d: %v", res.Points, res.Checks)
	}
}


