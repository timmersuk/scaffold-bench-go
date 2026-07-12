package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestToolsReadWriteLsEdit(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// write
	res, err := ExecuteTool(ctx, "write", `{"path":"foo.txt","content":"hello world"}`, dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if res != "created foo.txt" {
		t.Fatalf("unexpected write result: %s", res)
	}

	// read
	res, err = ExecuteTool(ctx, "read", `{"path":"foo.txt"}`, dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if res != "hello world" {
		t.Fatalf("unexpected read result: %s", res)
	}

	// edit
	res, err = ExecuteTool(ctx, "edit", `{"path":"foo.txt","old_str":"hello","new_str":"goodbye"}`, dir)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if res != "ok" {
		t.Fatalf("unexpected edit result: %s", res)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "foo.txt"))
	if string(content) != "goodbye world" {
		t.Fatalf("unexpected content after edit: %s", string(content))
	}

	// ls
	res, err = ExecuteTool(ctx, "ls", `{"path":"."}`, dir)
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if res != `["foo.txt"]` {
		t.Fatalf("unexpected ls result: %s", res)
	}
}

func TestEditCreatesFile(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	res, err := ExecuteTool(ctx, "edit", `{"path":"bar/baz.txt","old_str":"","new_str":"new"}`, dir)
	if err != nil {
		t.Fatalf("edit create: %v", err)
	}
	if res != "created bar/baz.txt" {
		t.Fatalf("unexpected result: %s", res)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "bar", "baz.txt"))
	if string(data) != "new" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestToolPathEscapes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if _, err := ExecuteTool(ctx, "read", `{"path":"../secret.txt"}`, dir); err == nil {
		t.Fatal("expected escape error")
	}
	if _, err := ExecuteTool(ctx, "read", `{"path":"/etc/passwd"}`, dir); err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestUnknownTool(t *testing.T) {
	ctx := context.Background()
	if _, err := ExecuteTool(ctx, "nope", `{}`, t.TempDir()); err == nil {
		t.Fatal("expected unknown tool error")
	}
}
