package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

// DefaultMaxTurns is the maximum number of agent turns.
const DefaultMaxTurns = 20

// Config controls a single agent run.
type Config struct {
	WorkDir      string
	Prompt       string
	Endpoint     string
	Model        string
	APIKey       string
	SystemPrompt string
	Harness      string
	Timeout      time.Duration
	MaxTurns     int
	OnEvent      func(model.RuntimeEvent)
	Caller       Caller
}

// Output is the result of an agent run.
type Output struct {
	Stdout       string
	ToolCalls    []model.ToolCall
	WallTimeMs   int64
	FirstTokenMs *int64
	Error        string
	ModelMetrics model.ModelMetrics
}

// Run executes the agent loop up to MaxTurns turns.
func Run(ctx context.Context, cfg Config) Output {
	if cfg.Model == "" {
		return Output{Error: "no model specified"}
	}
	if cfg.MaxTurns == 0 {
		cfg.MaxTurns = DefaultMaxTurns
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Minute
	}
	caller := cfg.Caller
	if caller == nil {
		caller = NewHTTPCaller()
	}

	harness := ResolveHarness(cfg.Harness)
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = DefaultSystemPrompt
	}
	preparedPrompt, requestTools := harness.Prepare(systemPrompt, OpenAITools())

	state := newRunState(preparedPrompt, cfg.Model)
	startedAt := time.Now()
	deadline := startedAt.Add(cfg.Timeout)
	state.conversation = append(state.conversation, ChatMessage{Role: "user", Content: cfg.Prompt})

	for turn := 0; turn < cfg.MaxTurns; turn++ {
		if ctx.Err() != nil {
			return finishRun(state, startedAt, "ABORTED")
		}
		if time.Now().After(deadline) {
			return finishRun(state, startedAt, "TIMEOUT")
		}

		reply, err := caller.Call(ctx, cfg.Endpoint, cfg.Model, cfg.APIKey, state.conversation, requestTools, func(delta string) {
			if state.firstTokenMs == nil && strings.TrimSpace(delta) != "" {
				elapsed := time.Since(startedAt).Milliseconds()
				state.firstTokenMs = &elapsed
			}
			emit(cfg.OnEvent, model.RuntimeEvent{Type: model.EventAssistantDelta, Delta: delta})
		})
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "context deadline exceeded") || strings.Contains(msg, "TIMEOUT") {
				return finishRun(state, startedAt, "TIMEOUT")
			}
			return finishRun(state, startedAt, fmt.Sprintf("CRASH: %s", msg))
		}

		applyModelCallMetrics(&state.metrics, reply.Metrics)
		emit(cfg.OnEvent, model.RuntimeEvent{Type: model.EventModelMetrics, Metrics: &state.metrics})

		content := reply.Message.Content
		toolCalls := reply.Message.ToolCalls
		if harness.Name() != HarnessNative && len(toolCalls) == 0 {
			parsedContent, parsedCalls := harness.Parse(stripThink(content))
			content = parsedContent
			toolCalls = make([]OpenAIToolCall, len(parsedCalls))
			for i, tc := range parsedCalls {
				toolCalls[i] = OpenAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: tc.Name, Arguments: tc.Arguments},
				}
			}
		}

		reply.Message.Content = content
		reply.Message.ToolCalls = toolCalls
		state.conversation = append(state.conversation, reply.Message)
		if content != "" {
			state.transcript = append(state.transcript, "assistant: "+content)
			emit(cfg.OnEvent, model.RuntimeEvent{Type: model.EventAssistant, Content: content})
		}

		if reply.FinishReason != FinishToolCalls || len(toolCalls) == 0 {
			emptyContent := strings.TrimSpace(content) == ""
			noToolCalls := len(toolCalls) == 0
			if emptyContent && noToolCalls && state.emptyNudges < 1 {
				state.emptyNudges++
				reasonHint := ""
				if strings.TrimSpace(reply.Reasoning) != "" {
					reasonHint = " Your previous turn ended inside reasoning without producing output."
				}
				state.transcript = append(state.transcript, "guard: empty assistant turn; nudging")
				nudge := ChatMessage{Role: "user", Content: "Continue." + reasonHint + " Emit either a final answer or a tool call."}
				state.conversation = append(state.conversation, nudge)
				continue
			}
			return finishRun(state, startedAt, "")
		}

		openAICalls := make([]OpenAIToolCall, len(toolCalls))
		for i, call := range toolCalls {
			openAICalls[i] = call
			tc := model.ToolCall{Name: call.Function.Name, Args: call.Function.Arguments, Turn: len(state.toolCalls)}
			state.toolCalls = append(state.toolCalls, tc)
			emit(cfg.OnEvent, model.RuntimeEvent{Type: model.EventToolCall, Call: &tc})
		}

		results, err := ExecuteToolBatch(ctx, openAICalls, cfg.WorkDir)
		if err != nil {
			return finishRun(state, startedAt, fmt.Sprintf("CRASH: tool execution: %s", err))
		}
		for i, call := range openAICalls {
			state.toolCalls[len(state.toolCalls)-len(openAICalls)+i].Result = results[i]
			emit(cfg.OnEvent, model.RuntimeEvent{Type: model.EventToolResult, Call: &state.toolCalls[len(state.toolCalls)-len(openAICalls)+i], Result: results[i]})
			state.conversation = append(state.conversation, ChatMessage{
				Role:       "tool",
				Content:    results[i],
				ToolCallID: call.ID,
			})
		}
	}

	state.transcript = append(state.transcript, fmt.Sprintf("guard: hit %d tool iterations, stopping", cfg.MaxTurns))
	return finishRun(state, startedAt, "")
}

type runState struct {
	conversation  []ChatMessage
	transcript    []string
	toolCalls     []model.ToolCall
	metrics       model.ModelMetrics
	emptyNudges   int
	firstTokenMs  *int64
}

func newRunState(systemPrompt, modelID string) *runState {
	return &runState{
		conversation: []ChatMessage{{Role: "system", Content: systemPrompt}},
		transcript:   []string{},
		toolCalls:    []model.ToolCall{},
		metrics: model.ModelMetrics{
			Model: modelID,
		},
	}
}

func finishRun(state *runState, startedAt time.Time, err string) Output {
	wallTimeMs := time.Since(startedAt).Milliseconds()
	out := Output{
		Stdout:       strings.Join(state.transcript, "\n"),
		ToolCalls:    append([]model.ToolCall(nil), state.toolCalls...),
		WallTimeMs:   wallTimeMs,
		FirstTokenMs: state.firstTokenMs,
		ModelMetrics: state.metrics,
	}
	if err != "" {
		out.Error = err
	}
	return out
}

func applyModelCallMetrics(target *model.ModelMetrics, source model.ModelMetrics) {
	target.RequestCount += source.RequestCount
	target.PromptTokens += source.PromptTokens
	target.CompletionTokens += source.CompletionTokens
	target.TotalTokens += source.TotalTokens
	target.TotalRequestTimeMs += source.TotalRequestTimeMs
	if source.PromptEvalTokens != 0 {
		target.PromptEvalTokens += source.PromptEvalTokens
	}
	if source.PromptEvalTimeMs != 0 {
		target.PromptEvalTimeMs += source.PromptEvalTimeMs
	}
	if source.CompletionEvalTokens != 0 {
		target.CompletionEvalTokens += source.CompletionEvalTokens
	}
	if source.CompletionEvalTimeMs != 0 {
		target.CompletionEvalTimeMs += source.CompletionEvalTimeMs
	}
	target.Requests = append(target.Requests, source.Requests...)
}

func stripThink(content string) string {
	for {
		start := strings.Index(content, "<think>")
		if start == -1 {
			return content
		}
		end := strings.Index(content[start:], "</think>")
		if end == -1 {
			return content[:start]
		}
		content = content[:start] + content[start+end+len("</think>"):]
	}
}

func emit(fn func(model.RuntimeEvent), e model.RuntimeEvent) {
	if fn != nil {
		fn(e)
	}
}

// DefaultSystemPrompt is a generic prompt for the benchmark agent.
const DefaultSystemPrompt = `You are a helpful software engineering assistant. You can use the provided tools to inspect and edit files in the workspace.

Guidelines:
- Only use relative paths; absolute paths are not allowed.
- Use read/ls to understand the codebase before editing.
- Use edit for small surgical changes and write for creating new files.
- Use bash for search and verification commands.
- Prefer concise, correct changes.`
