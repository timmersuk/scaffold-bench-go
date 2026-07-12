package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifests(t *testing.T) {
	tmp := t.TempDir()
	scenarioDir := filepath.Join(tmp, "demo")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `id: demo
name: create-hello
category: basic
family: regex-style
difficulty: low
rubricKind: 10pt
signalType: stdout
track: execution
prompt: |
  Create a file at playground/hello.txt containing exactly the text "hello".
workspace:
  root: playground
rubric:
  correctness:
    - name: contains hello
      type: file_contains
      weight: 8
      params:
        file: playground/hello.txt
        pattern: "^hello$"
  scope:
    - name: only hello
      type: files_changed_only
      weight: 2
      params:
        allowed:
          - playground/hello.txt
labels:
  pass: pass
  partial: partial
  fail: fail
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	scenarios, err := LoadManifests(tmp)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(scenarios))
	}
	s := scenarios[0]
	if s.ID != "demo" {
		t.Errorf("expected id demo, got %s", s.ID)
	}
	if s.Name != "create-hello" {
		t.Errorf("expected name create-hello, got %s", s.Name)
	}
	if s.MaxPoints != 10 {
		t.Errorf("expected 10 max points, got %d", s.MaxPoints)
	}
	if s.Manifest.Workspace.Root != "playground" {
		t.Errorf("expected workspace root playground, got %s", s.Manifest.Workspace.Root)
	}
	if len(s.Manifest.Rubric.Correctness) != 1 {
		t.Errorf("expected 1 correctness check, got %d", len(s.Manifest.Rubric.Correctness))
	}
}

func TestManifestNestedMeta(t *testing.T) {
	tmp := t.TempDir()
	scenarioDir := filepath.Join(tmp, "nested")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := `meta:
  id: nested
  name: nested-scenario
  category: test
  family: test-family
  difficulty: low
  rubricKind: 10pt
  signalType: trace
  prompt: test
workspace:
  root: playground
rubric:
  correctness: []
  scope: []
  pattern: []
  verification: []
  cleanup: []
labels:
  pass: pass
  partial: partial
  fail: fail
`
	if err := os.WriteFile(filepath.Join(scenarioDir, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	scenarios, err := LoadManifests(tmp)
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	if len(scenarios) != 1 || scenarios[0].ID != "nested" {
		t.Fatalf("expected nested scenario, got %+v", scenarios)
	}
}

func TestNewRegistryLoadsDemo(t *testing.T) {
	registry := NewRegistry()
	s, ok := registry.Get("demo")
	if !ok {
		t.Fatal("demo scenario not found")
	}
	if s.ID != "demo" || s.MaxPoints != 10 {
		t.Fatalf("unexpected demo scenario: %+v", s)
	}
}
