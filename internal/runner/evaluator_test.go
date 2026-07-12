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
