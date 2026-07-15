package oneshot

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// LoadLabPrompts reads all markdown files from the given directory and returns
// them as LabPrompt values. Files are sorted alphabetically by name.
func LoadLabPrompts(dir string) ([]model.LabPrompt, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read lab_prompts dir: %w", err)
	}

	var prompts []model.LabPrompt
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		path := filepath.Join(dir, name)
		p, err := loadLabPrompt(path, name)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		prompts = append(prompts, p)
	}

	sort.Slice(prompts, func(i, j int) bool { return prompts[i].ID < prompts[j].ID })
	return prompts, nil
}

func loadLabPrompt(path, filename string) (model.LabPrompt, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.LabPrompt{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return model.LabPrompt{}, fmt.Errorf("missing frontmatter start")
	}

	var frontmatter strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteByte('\n')
	}

	var meta struct {
		Title    string `yaml:"title"`
		Category string `yaml:"category"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter.String()), &meta); err != nil {
		return model.LabPrompt{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if meta.Title == "" || meta.Category == "" {
		return model.LabPrompt{}, fmt.Errorf("frontmatter must have title and category")
	}

	var body strings.Builder
	for scanner.Scan() {
		body.WriteString(scanner.Text())
		body.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return model.LabPrompt{}, fmt.Errorf("read body: %w", err)
	}

	id := strings.TrimSuffix(filename, filepath.Ext(filename))
	return model.LabPrompt{
		ID:       id,
		Title:    meta.Title,
		Category: meta.Category,
		Prompt:   strings.TrimSpace(body.String()),
	}, nil
}
