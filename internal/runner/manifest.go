package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
	"gopkg.in/yaml.v3"
)

// Manifest is the parsed scenario definition.
type Manifest struct {
	Meta           Meta          `yaml:"-"`
	Workspace      Workspace     `yaml:"workspace"`
	Requires       []string      `yaml:"requires,omitempty"`
	Setup          *Setup        `yaml:"setup,omitempty"`
	HiddenFixtures []FileMapping `yaml:"hiddenFixtures,omitempty"`
	Build          *Build        `yaml:"build,omitempty"`
	Rubric         Rubric        `yaml:"rubric"`
	Labels         Labels        `yaml:"labels"`
}

// Meta holds the scenario metadata.
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

// Workspace describes the workspace layout for a scenario.
type Workspace struct {
	Root         string   `yaml:"root"`
	PristineDir  string   `yaml:"pristineDir,omitempty"`
	AllowedPaths []string `yaml:"allowedPaths,omitempty"`
}

// Setup describes optional files copied into the workspace before the run.
type Setup struct {
	Files []FileMapping `yaml:"files,omitempty"`
}

// FileMapping maps a source file in the scenario directory to a workspace path.
type FileMapping struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
}

// Build holds pre-check build commands.
type Build struct {
	Commands []string          `yaml:"commands,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
}

// Rubric is the full set of checks grouped by axis.
type Rubric struct {
	Correctness  []Check `yaml:"correctness"`
	Scope          []Check `yaml:"scope"`
	Pattern        []Check `yaml:"pattern"`
	Verification   []Check `yaml:"verification"`
	Cleanup        []Check `yaml:"cleanup"`
}

// Check is a single rubric check.
type Check struct {
	Name   string         `yaml:"name"`
	Type   string         `yaml:"type"`
	Weight int            `yaml:"weight"`
	Params map[string]any `yaml:"params"`
	OnSkip string         `yaml:"onSkip,omitempty"`
}

// Labels provide human-readable result summaries.
type Labels struct {
	Pass    string `yaml:"pass"`
	Partial string `yaml:"partial"`
	Fail    string `yaml:"fail"`
}

// manifestEnvelope is used for YAML parsing. Meta fields may appear either
// under a `meta` key or inline at the top level of the manifest.
type manifestEnvelope struct {
	Meta *Meta `yaml:"meta,omitempty"`

	ID         string `yaml:"id"`
	Name       string `yaml:"name"`
	Category   string `yaml:"category"`
	Family     string `yaml:"family"`
	Difficulty string `yaml:"difficulty"`
	RubricKind string `yaml:"rubricKind"`
	SignalType string `yaml:"signalType"`
	Track      string `yaml:"track,omitempty"`
	Prompt     string `yaml:"prompt"`

	Workspace      Workspace     `yaml:"workspace"`
	Requires       []string      `yaml:"requires,omitempty"`
	Setup          *Setup        `yaml:"setup,omitempty"`
	HiddenFixtures []FileMapping `yaml:"hiddenFixtures,omitempty"`
	Build          *Build        `yaml:"build,omitempty"`
	Rubric         Rubric        `yaml:"rubric"`
	Labels         Labels        `yaml:"labels"`
}

// UnmarshalYAML implements custom YAML unmarshalling so that both the
// explicitly nested `meta:` block and the inline top-level meta keys work.
func (m *Manifest) UnmarshalYAML(unmarshal func(any) error) error {
	var env manifestEnvelope
	if err := unmarshal(&env); err != nil {
		return err
	}

	if env.Meta != nil {
		m.Meta = *env.Meta
	} else {
		m.Meta = Meta{
			ID:         env.ID,
			Name:       env.Name,
			Category:   env.Category,
			Family:     env.Family,
			Difficulty: env.Difficulty,
			RubricKind: env.RubricKind,
			SignalType: env.SignalType,
			Track:      env.Track,
			Prompt:     env.Prompt,
		}
	}
	m.Workspace = env.Workspace
	m.Requires = env.Requires
	m.Setup = env.Setup
	m.HiddenFixtures = env.HiddenFixtures
	m.Build = env.Build
	m.Rubric = env.Rubric
	m.Labels = env.Labels
	return nil
}

// Input is everything the evaluator needs to score a run.
type Input struct {
	Manifest    Manifest
	WorkDir     string
	PristineDir string
	Dir         string // scenario directory, used to resolve hidden fixtures
	ToolCalls   []model.ToolCall
}

// LoadManifests walks root for `manifest.yaml` files and converts each to a Scenario.
func LoadManifests(root string) ([]Scenario, error) {
	if !filepath.IsAbs(root) {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			// When running from a sub-package (e.g. tests), fall back to the
			// repository root so relative paths still resolve.
			if r, err := repoRoot(); err == nil {
				root = filepath.Join(r, root)
			}
		}
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("scenarios root %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scenarios root %q is not a directory", root)
	}

	var scenarios []Scenario
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Base(path)) != "manifest.yaml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var m Manifest
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		scenarioDir := filepath.Dir(path)
		scenario, err := manifestToScenario(scenarioDir, m)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", path, err)
		}
		scenarios = append(scenarios, scenario)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(scenarios, func(i, j int) bool { return scenarios[i].ID < scenarios[j].ID })
	return scenarios, nil
}

func manifestToScenario(dir string, m Manifest) (Scenario, error) {
	id := m.Meta.ID
	if id == "" {
		id = filepath.Base(dir)
	}
	if m.Meta.Name == "" {
		return Scenario{}, fmt.Errorf("scenario %s: missing name", id)
	}

	s := Scenario{
		ID:         id,
		Name:       m.Meta.Name,
		Category:   m.Meta.Category,
		Family:     m.Meta.Family,
		RubricKind: m.Meta.RubricKind,
		Prompt:     m.Meta.Prompt,
		Dir:        dir,
		Manifest:   m,
	}

	if m.Workspace.PristineDir != "" {
		s.PristineDir = filepath.Join(dir, m.Workspace.PristineDir)
		if info, err := os.Stat(s.PristineDir); err != nil || !info.IsDir() {
			// Treat a missing pristine directory as empty; the evaluator
			// will diff against an empty workspace.
			if _, err := os.Stat(s.PristineDir); os.IsNotExist(err) {
				s.PristineDir = ""
			}
		}
	}

	// If the workspace root exists as a directory under the scenario directory,
	// use it as the source of starter files.
	if m.Workspace.Root != "" {
		rootDir := filepath.Join(dir, m.Workspace.Root)
		if info, err := os.Stat(rootDir); err == nil && info.IsDir() {
			s.WorkspaceSource = rootDir
		}
	}

	s.MaxPoints = maxPointsFor(m)
	return s, nil
}

func maxPointsFor(m Manifest) int {
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

// repoRoot locates the repository root by walking up until go.mod is found.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found")
}
