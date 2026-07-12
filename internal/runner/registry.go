package runner

// Scenario is a single benchmark scenario.
type Scenario struct {
	ID              string
	Name            string
	Category        string
	Family          string
	RubricKind      string
	MaxPoints       int
	Prompt          string
	WorkspaceSource string // path to copy into the workspace root; empty means none
	PristineDir     string // path to diff against; empty means empty pristine
	Dir             string // path to the scenario directory on disk
	Manifest        Manifest
}

// Registry is a minimal in-memory scenario registry.
type Registry struct {
	scenarios map[string]Scenario
}

// NewRegistry returns the built-in scenario registry loaded from the scenarios/ directory.
func NewRegistry() *Registry {
	scenarios, err := LoadManifests("scenarios")
	if err != nil {
		panic(err)
	}
	m := make(map[string]Scenario, len(scenarios))
	for _, s := range scenarios {
		m[s.ID] = s
	}
	return &Registry{scenarios: m}
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
