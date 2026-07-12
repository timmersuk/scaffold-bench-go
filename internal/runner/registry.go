package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// Scenario is a single benchmark scenario.
type Scenario struct {
	ID              string
	Name            string
	Category        string
	Family          string
	RubricKind      string
	MaxPoints       int
	Prompt          string
	WorkspaceSource string // path to copy into the temp workspace; empty means none
	PristineDir     string // path to diff against; empty means empty pristine
	Evaluator       func(ctx context.Context, workDir string, toolCalls []model.ToolCall) model.Evaluation
}

// Registry is a minimal in-memory scenario registry.
type Registry struct {
	scenarios map[string]Scenario
}

// NewRegistry returns the built-in scenario registry.
func NewRegistry() *Registry {
	return &Registry{
		scenarios: map[string]Scenario{
			"demo": {
				ID:         "demo",
				Name:       "create-hello",
				Category:   "basic",
				Family:     "regex-style",
				RubricKind: "10pt",
				MaxPoints:  10,
				Prompt: `Create a file at playground/hello.txt containing exactly the text "hello" (lowercase, no extra whitespace). Then finish.`,
				Evaluator: func(_ context.Context, workDir string, _ []model.ToolCall) model.Evaluation {
					path := workDir + "/playground/hello.txt"
					data, err := os.ReadFile(path)
					if err != nil {
						return model.Evaluation{
							Status:    "fail",
							Points:    0,
							MaxPoints: 10,
							Summary:   fmt.Sprintf("File not created: %v", err),
							Checks: []model.CheckResult{
								{Name: "file exists", Pass: false, Weight: 10, Detail: err.Error()},
							},
						}
					}
					content := strings.TrimSpace(string(data))
					if content == "hello" {
						return model.Evaluation{
							Status:    "pass",
							Points:    10,
							MaxPoints: 10,
							Summary:   "Created playground/hello.txt with 'hello'.",
							Checks: []model.CheckResult{
								{Name: "file exists", Pass: true, Weight: 2},
								{Name: "content matches", Pass: true, Weight: 8},
							},
						}
					}
					return model.Evaluation{
						Status:    "fail",
						Points:    0,
						MaxPoints: 10,
						Summary:   fmt.Sprintf("File content %q does not match expected 'hello'", content),
						Checks: []model.CheckResult{
							{Name: "file exists", Pass: true, Weight: 2},
							{Name: "content matches", Pass: false, Weight: 8, Detail: content},
						},
					}
				},
			},
		},
	}
}

// Get returns a scenario by ID.
func (r *Registry) Get(id string) (Scenario, bool) {
	s, ok := r.scenarios[id]
	return s, ok
}

// IDs returns all registered scenario IDs.
func (r *Registry) IDs() []string {
	var ids []string
	for id := range r.scenarios {
		ids = append(ids, id)
	}
	sortStrings(ids)
	return ids
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
