package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// copyDirOverwrite copies src to dst recursively, replacing existing files.
func copyDirOverwrite(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0o644)
	})
}

func writeWorkspaceFile(t *testing.T, dir, relPath string, content []byte) {
	t.Helper()
	p := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

func runAstCheck(t *testing.T, checkType string, params map[string]any, files map[string][]byte) (bool, string) {
	t.Helper()
	tmp := t.TempDir()
	for rel, data := range files {
		writeWorkspaceFile(t, tmp, rel, data)
	}

	m := makeManifest(Rubric{
		Correctness: []Check{{Name: checkType, Type: checkType, Weight: 1, Params: params}},
	})

	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: tmp})
	if len(res.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(res.Checks))
	}
	return res.Checks[0].Pass, res.Checks[0].Detail
}

const routeWithLoader = `
import { createFileRoute } from "@tanstack/react-router";
import { fetchProjects } from "../apiClient";
import { ProjectsTable } from "../components/ProjectsTable";

export const Route = createFileRoute("/projects")({
  loader: async () => {
    const projects = await fetchProjects();
    return { projects };
  },
  component: ProjectsPage,
});

function ProjectsPage() {
  const { projects } = Route.useLoaderData();
  return <ProjectsTable projects={projects} />;
}
`

const routeWithoutLoaderData = `
import { createFileRoute } from "@tanstack/react-router";
import { fetchProjects } from "../apiClient";
import { ProjectsTable } from "../components/ProjectsTable";

export const Route = createFileRoute("/projects")({
  loader: async () => {
    const projects = await fetchProjects();
    return { projects };
  },
  component: ProjectsPage,
});

function ProjectsPage() {
  return <ProjectsTable />;
}
`

func TestAstPropertyContainsCallPass(t *testing.T) {
	pass, detail := runAstCheck(t, "ast_property_contains_call", map[string]any{
		"file":     "src/routes/projects.tsx",
		"property": "loader",
		"callee":   "fetchProjects",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(routeWithLoader)})
	if !pass {
		t.Fatalf("expected pass, got fail: %s", detail)
	}
}

func TestAstPropertyContainsCallFail(t *testing.T) {
	route := `
export const Route = createFileRoute("/projects")({
  loader: async () => ({ projects: [] }),
  component: ProjectsPage,
});
`
	pass, detail := runAstCheck(t, "ast_property_contains_call", map[string]any{
		"file":     "src/routes/projects.tsx",
		"property": "loader",
		"callee":   "fetchProjects",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(route)})
	if pass {
		t.Fatalf("expected fail, got pass: %s", detail)
	}
}

func TestAstFileCallsPass(t *testing.T) {
	pass, detail := runAstCheck(t, "ast_file_calls", map[string]any{
		"file":   "src/routes/projects.tsx",
		"callee": "useLoaderData",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(routeWithLoader)})
	if !pass {
		t.Fatalf("expected pass, got fail: %s", detail)
	}
}

func TestAstFileCallsFail(t *testing.T) {
	pass, detail := runAstCheck(t, "ast_file_calls", map[string]any{
		"file":   "src/routes/projects.tsx",
		"callee": "useLoaderData",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(routeWithoutLoaderData)})
	if pass {
		t.Fatalf("expected fail, got pass: %s", detail)
	}
}

func TestAstJsxPassesPropPass(t *testing.T) {
	pass, detail := runAstCheck(t, "ast_jsx_passes_prop", map[string]any{
		"file":      "src/routes/projects.tsx",
		"component": "ProjectsTable",
		"prop":      "projects",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(routeWithLoader)})
	if !pass {
		t.Fatalf("expected pass, got fail: %s", detail)
	}
}

func TestAstJsxPassesPropFail(t *testing.T) {
	pass, detail := runAstCheck(t, "ast_jsx_passes_prop", map[string]any{
		"file":      "src/routes/projects.tsx",
		"component": "ProjectsTable",
		"prop":      "projects",
	}, map[string][]byte{"src/routes/projects.tsx": []byte(routeWithoutLoaderData)})
	if pass {
		t.Fatalf("expected fail, got pass: %s", detail)
	}
}

func TestAstUnsupportedType(t *testing.T) {
	m := makeManifest(Rubric{
		Correctness: []Check{
			{Name: "unsupported", Type: "ast_call_count", Weight: 3, Params: map[string]any{}},
		},
	})
	ev := NewEvaluator()
	res := ev.Evaluate(context.Background(), Input{Manifest: m, WorkDir: t.TempDir()})
	if res.Points != 0 {
		t.Fatalf("expected 0 points for unsupported ast check, got %d: %v", res.Points, res.Checks)
	}
	if res.Checks[0].Detail == "" {
		t.Fatalf("expected unsupported detail, got empty")
	}
}

func loadScenario(t *testing.T, id string) Scenario {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	scenarios, err := LoadManifests(filepath.Join(root, "scenarios"))
	if err != nil {
		t.Fatalf("load manifests: %v", err)
	}
	for _, s := range scenarios {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("scenario %s not found", id)
	return Scenario{}
}

func evaluateSB25(t *testing.T, gold bool) model.Evaluation {
	t.Helper()
	scenario := loadScenario(t, "SB-25")

	workDir := t.TempDir()
	wsRoot := scenario.Manifest.Workspace.Root
	destRoot := filepath.Join(workDir, wsRoot)
	if err := copyDir(scenario.WorkspaceSource, destRoot); err != nil {
		t.Fatalf("copy workspace: %v", err)
	}

	if gold {
		// _fixtures/gold holds the workspace-root contents directly.
		goldSrc := filepath.Join(scenario.Dir, "_fixtures", "gold", "tanstack-router-app")
		if err := copyDirOverwrite(goldSrc, destRoot); err != nil {
			t.Fatalf("copy gold fixtures: %v", err)
		}
	}

	toolCalls := []model.ToolCall{
		{Name: "read", Args: `{"path":"playground/tanstack-router-app/src/routes/projects.tsx"}`},
		{Name: "read", Args: `{"path":"playground/tanstack-router-app/src/components/ProjectsTable.tsx"}`},
		{Name: "edit", Args: `{"path":"playground/tanstack-router-app/src/routes/projects.tsx"}`},
		{Name: "edit", Args: `{"path":"playground/tanstack-router-app/src/components/ProjectsTable.tsx"}`},
	}

	ev := NewEvaluator()
	return ev.Evaluate(context.Background(), Input{
		Manifest:    scenario.Manifest,
		WorkDir:     workDir,
		PristineDir: scenario.PristineDir,
		Dir:         scenario.Dir,
		ToolCalls:   toolCalls,
	})
}

func TestSB25GoldGate(t *testing.T) {
	res := evaluateSB25(t, true)
	if res.Status != "pass" {
		t.Fatalf("expected gold to pass, got %s: %v", res.Status, res.Checks)
	}
	if res.Points != res.MaxPoints {
		t.Fatalf("expected %d points, got %d", res.MaxPoints, res.Points)
	}
}

func TestSB25BrokenGate(t *testing.T) {
	res := evaluateSB25(t, false)
	if res.Status == "pass" {
		t.Fatalf("expected broken not to pass, got pass: %v", res.Checks)
	}
	if res.Breakdown.Correctness > 1 {
		t.Fatalf("expected correctness <= 1 for broken, got %d", res.Breakdown.Correctness)
	}
}
