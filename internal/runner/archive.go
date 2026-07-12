package runner

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// captureWorkspace returns the diff between workDir and pristineDir.
// Files are compared relative to each root; entries are prefixed with rootName.
func captureWorkspace(workDir, pristineDir, rootName string) (*model.WorkspaceArchive, error) {
	currentFiles, err := walkFiles(workDir)
	if err != nil {
		return nil, fmt.Errorf("walk current workspace: %w", err)
	}
	pristineFiles, err := walkFiles(pristineDir)
	if err != nil {
		// A missing pristine directory is treated as empty.
		pristineFiles = map[string]string{}
	}

	archive := &model.WorkspaceArchive{Version: 1}
	for rel, content := range currentFiles {
		path := filepath.ToSlash(filepath.Join(rootName, rel))
		pristine, exists := pristineFiles[rel]
		if !exists || pristine != content {
			archive.Changed = append(archive.Changed, model.WorkspaceArchiveEntry{
				Path:    path,
				Content: content,
			})
		}
	}
	for rel := range pristineFiles {
		if _, exists := currentFiles[rel]; !exists {
			archive.Deleted = append(archive.Deleted, filepath.ToSlash(filepath.Join(rootName, rel)))
		}
	}
	sort.Strings(archive.Deleted)
	sort.Slice(archive.Changed, func(i, j int) bool { return archive.Changed[i].Path < archive.Changed[j].Path })
	return archive, nil
}

func walkFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	if root == "" {
		return files, nil
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", root)
	}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		// Treat non-UTF-8 files as unchanged/skipped.
		if !isValidUTF8(data) {
			return nil
		}
		files[rel] = strings.ReplaceAll(string(data), "\r\n", "\n")
		return nil
	})
	return files, err
}

func isValidUTF8(b []byte) bool {
	return strings.ToValidUTF8(string(b), "") == string(b)
}

func archiveArtifactPath(dataDir, runID, scenarioID string) string {
	return filepath.Join(dataDir, "artifacts", runID, scenarioID+".json")
}

func writeArtifact(path string, archive *model.WorkspaceArchive) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(archive)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
