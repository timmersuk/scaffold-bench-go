package agent

import (
	"context"
	"testing"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

type fakeCaller struct {
	responses []ModelResponse
}

func (f *fakeCaller) Call(ctx context.Context, endpoint, model, apiKey string, messages []ChatMessage, tools []ToolDefinition, onDelta func(string), onReasoningDelta func(string)) (ModelResponse, error) {
	if len(f.responses) == 0 {
		return ModelResponse{FinishReason: FinishStop, Message: ChatMessage{Role: "assistant", Content: "done"}}, nil
	}
	r := f.responses[0]
	f.responses = f.responses[1:]
	if onDelta != nil && r.Message.Content != "" {
		onDelta(r.Message.Content)
	}
	return r, nil
}

func TestAgentLoopWithToolCall(t *testing.T) {
	calls := &fakeCaller{
		responses: []ModelResponse{
			{
				FinishReason: FinishToolCalls,
				Message: ChatMessage{
					Role: "assistant",
					ToolCalls: []OpenAIToolCall{
						{ID: "call_1", Type: "function", Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: "write", Arguments: `{"path":"test.txt","content":"ok"}`}},
					},
				},
			},
			{
				FinishReason: FinishStop,
				Message:      ChatMessage{Role: "assistant", Content: "finished"},
			},
		},
	}

	var events []model.RuntimeEvent
	out := Run(context.Background(), Config{
		WorkDir: t.TempDir(),
		Prompt:  "write a file",
		Model:   "fake",
		Caller:  calls,
		OnEvent: func(ev model.RuntimeEvent) { events = append(events, ev) },
	})

	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].Name != "write" {
		t.Fatalf("expected one write tool call, got %+v", out.ToolCalls)
	}
	if out.ToolCalls[0].Result != "created test.txt" {
		t.Fatalf("unexpected tool result: %s", out.ToolCalls[0].Result)
	}

	var sawDelta, sawToolCall, sawToolResult, sawAssistant bool
	for _, ev := range events {
		switch ev.Type {
		case model.EventAssistantDelta:
			sawDelta = true
		case model.EventToolCall:
			sawToolCall = true
		case model.EventToolResult:
			sawToolResult = true
		case model.EventAssistant:
			sawAssistant = true
		}
	}
	if !sawToolCall || !sawToolResult || !sawAssistant {
		t.Fatalf("missing events: delta=%v toolCall=%v toolResult=%v assistant=%v", sawDelta, sawToolCall, sawToolResult, sawAssistant)
	}
}

func TestAgentLoopNudgeOnEmpty(t *testing.T) {
	calls := &fakeCaller{
		responses: []ModelResponse{
			{FinishReason: FinishStop, Message: ChatMessage{Role: "assistant"}},
			{FinishReason: FinishStop, Message: ChatMessage{Role: "assistant", Content: "done"}},
		},
	}

	out := Run(context.Background(), Config{
		WorkDir: t.TempDir(),
		Prompt:  "do nothing",
		Model:   "fake",
		Caller:  calls,
	})
	if out.Error != "" {
		t.Fatalf("unexpected error: %s", out.Error)
	}
	if !contains(out.Stdout, "guard: empty assistant turn") {
		t.Fatalf("expected nudge in stdout: %s", out.Stdout)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
