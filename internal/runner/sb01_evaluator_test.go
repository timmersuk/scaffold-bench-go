package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

var brokenThrottle = `function throttle(fn, ms) {
  let timer;
  return function (...args) {
    clearTimeout(timer);
    timer = setTimeout(() => fn.apply(this, args), ms);
  };
}`

var fixedThrottle = `function throttle(fn, ms) {
  let last = 0;
  return function (...args) {
    const now = Date.now();
    if (now - last >= ms) {
      last = now;
      return fn.apply(this, args);
    }
  };
}`

func skipIfNoBun(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("bun not available on PATH")
	}
}

func loadSB01(t *testing.T) Scenario {
	t.Helper()
	scenarios, err := LoadManifests("scenarios")
	if err != nil {
		t.Fatalf("LoadManifests: %v", err)
	}
	for _, s := range scenarios {
		if s.ID == "SB-01" {
			return s
		}
	}
	t.Fatal("SB-01 scenario not found")
	return Scenario{}
}

func makeWorkDir(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	root := filepath.Join(tmp, "playground")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir playground: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "utils.js"), []byte(content), 0o644); err != nil {
		t.Fatalf("write utils.js: %v", err)
	}
	return tmp
}

func stageHiddenFixture(t *testing.T, scenario Scenario) string {
	t.Helper()
	tmp := t.TempDir()
	if len(scenario.Manifest.HiddenFixtures) == 0 {
		t.Fatal("SB-01 has no hidden fixtures")
	}
	hf := scenario.Manifest.HiddenFixtures[0]
	src := filepath.Join(scenario.Dir, hf.Src)
	dst := filepath.Join(tmp, hf.Dest)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read hidden fixture: %v", err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write hidden fixture: %v", err)
	}
	return tmp
}

func readPristine(t *testing.T, scenario Scenario) string {
	t.Helper()
	p := filepath.Join(scenario.PristineDir, "utils.js")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read pristine utils.js: %v", err)
	}
	return string(data)
}

var readThenEditUtils = []model.ToolCall{
	{Name: "read", Args: `{"path":"playground/utils.js"}`},
	{Name: "edit", Args: `{"path":"playground/utils.js","old_str":"function throttle","new_str":"function throttle"}`},
}

func TestSB01GoldWorkspace(t *testing.T) {
	skipIfNoBun(t)
	scenario := loadSB01(t)

	fixed := strings.Replace(readPristine(t, scenario), brokenThrottle, fixedThrottle, 1)
	workDir := makeWorkDir(t, fixed)
	hiddenDir := stageHiddenFixture(t, scenario)

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{
		Manifest:    scenario.Manifest,
		WorkDir:     workDir,
		PristineDir: scenario.PristineDir,
		Dir:         scenario.Dir,
		HiddenDir:   hiddenDir,
		ToolCalls:   readThenEditUtils,
	})

	if res.Status != "pass" {
		t.Fatalf("expected status pass, got %s: %+v", res.Status, res.Checks)
	}
	if res.Points != res.MaxPoints {
		t.Fatalf("expected %d points, got %d: %+v", res.MaxPoints, res.Points, res.Checks)
	}
}

func TestSB01UntouchedWorkspaceIsPartial(t *testing.T) {
	scenario := loadSB01(t)

	workDir := makeWorkDir(t, readPristine(t, scenario))
	hiddenDir := stageHiddenFixture(t, scenario)

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{
		Manifest:    scenario.Manifest,
		WorkDir:     workDir,
		PristineDir: scenario.PristineDir,
		Dir:         scenario.Dir,
		HiddenDir:   hiddenDir,
		ToolCalls:   nil,
	})

	if res.Status != "partial" {
		t.Fatalf("expected status partial, got %s: %+v", res.Status, res.Checks)
	}
	if res.Points != 7 {
		t.Fatalf("expected 7 points for untouched workspace, got %d: %+v", res.Points, res.Checks)
	}
}

func TestSB01DamagedWorkspaceFails(t *testing.T) {
	scenario := loadSB01(t)

	pristine := readPristine(t, scenario)
	// Change throttle to a no-op, alter debounce, add a console.log, and add
	// a comment. This should fail correctness, pattern, and cleanup.
	damaged := strings.Replace(pristine, brokenThrottle,
		`// added comment
function throttle(fn, ms) {
  console.log("bad");
  return function () {};
}`, 1)
	damaged = strings.Replace(damaged,
		"function debounce(fn, ms) {",
		"function debounce(fn, ms) {\n  console.log(\"bad\");", 1)
	damaged = strings.Replace(damaged,
		"function formatDate(date) {",
		"function formatDate(date) {\n  console.log(\"bad\");", 1)

	workDir := makeWorkDir(t, damaged)
	hiddenDir := stageHiddenFixture(t, scenario)

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{
		Manifest:    scenario.Manifest,
		WorkDir:     workDir,
		PristineDir: scenario.PristineDir,
		Dir:         scenario.Dir,
		HiddenDir:   hiddenDir,
		ToolCalls:   readThenEditUtils,
	})

	if res.Status != "fail" {
		t.Fatalf("expected status fail, got %s: %+v", res.Status, res.Checks)
	}
	if res.Points > 4 {
		t.Fatalf("expected <= 4 points for damaged workspace, got %d: %+v", res.Points, res.Checks)
	}
}
