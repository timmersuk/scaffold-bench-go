package agent

import (
	"strings"
	"testing"
)

func TestNativeHarness(t *testing.T) {
	h := ResolveHarness("native")
	prompt, tools := h.Prepare("system", OpenAITools())
	if prompt != "system" {
		t.Fatalf("unexpected prompt: %s", prompt)
	}
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}
	content, calls := h.Parse("hello")
	if content != "hello" || len(calls) != 0 {
		t.Fatalf("native parse failed")
	}
}

func TestHermesHarness(t *testing.T) {
	h := ResolveHarness("hermes")
	prompt, tools := h.Prepare("system", OpenAITools())
	if !strings.Contains(prompt, "<tools>") {
		t.Fatal("expected tools injected into prompt")
	}
	if tools != nil {
		t.Fatal("expected no native tools for hermes")
	}
	content, calls := h.Parse(`Thinking...<tool_call>{"name": "write", "arguments": {"path": "x.txt", "content": "hi"}}</tool_call>done`)
	if strings.TrimSpace(content) != "Thinking...done" {
		t.Fatalf("unexpected content: %q", content)
	}
	if len(calls) != 1 || calls[0].Name != "write" {
		t.Fatalf("unexpected calls: %+v", calls)
	}
}

func TestQwenHarnessArgumentsString(t *testing.T) {
	h := ResolveHarness("qwen")
	content, calls := h.Parse(`<function_call>{"name": "bash", "arguments": "ls"}</function_call>`)
	if content != "" {
		t.Fatalf("unexpected content: %q", content)
	}
	if len(calls) != 1 || calls[0].Name != "bash" || calls[0].Arguments != "ls" {
		t.Fatalf("unexpected calls: %+v", calls)
	}
}
