package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/timmersuk/scaffold-bench-go/internal/model"
)

const (
	temperature = 0.2
	topP        = 0.95
)

// FinishReason is why the model stopped generating.
type FinishReason string

const (
	FinishToolCalls    FinishReason = "tool_calls"
	FinishStop         FinishReason = "stop"
	FinishLength       FinishReason = "length"
	FinishContentFilter FinishReason = "content_filter"
)

// ModelResponse is the result of a single model call.
type ModelResponse struct {
	FinishReason FinishReason
	Message      ChatMessage
	Reasoning    string
	Metrics      model.ModelMetrics
}

// Caller abstracts the OpenAI-compatible chat completions endpoint.
type Caller interface {
	Call(ctx context.Context, endpoint, model, apiKey string, messages []ChatMessage, tools []ToolDefinition, onDelta func(string)) (ModelResponse, error)
}

// HTTPCaller makes streaming HTTP calls to an OpenAI-compatible endpoint.
type HTTPCaller struct {
	Client *http.Client
}

// NewHTTPCaller creates a caller with a default client.
func NewHTTPCaller() *HTTPCaller {
	return &HTTPCaller{Client: &http.Client{}}
}

// Call performs a streaming chat completion.
func (c *HTTPCaller) Call(ctx context.Context, endpoint, modelID, apiKey string, messages []ChatMessage, tools []ToolDefinition, onDelta func(string)) (ModelResponse, error) {
	endpoint = normalizeEndpoint(endpoint)
	body, err := json.Marshal(chatRequest{
		Model:       modelID,
		Messages:    messages,
		Temperature: temperature,
		TopP:        topP,
		Stream:      true,
		StreamOptions: streamOptions{IncludeUsage: true},
		Tools:       tools,
	})
	if err != nil {
		return ModelResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ModelResponse{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	if apiKey != "" {
		req.Header.Set("authorization", "Bearer "+apiKey)
	}

	startedAt := time.Now()
	resp, err := c.Client.Do(req)
	if err != nil {
		return ModelResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return ModelResponse{}, fmt.Errorf("model returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	stream, usage, timings, err := readChatStream(ctx, resp.Body, onDelta)
	if err != nil {
		return ModelResponse{}, err
	}

	requestTimeMs := time.Since(startedAt).Milliseconds()
	metrics := extractMetrics(modelID, usage, timings, requestTimeMs)

	return ModelResponse{
		FinishReason: stream.finishReason,
		Message: ChatMessage{
			Role:      "assistant",
			Content:   stream.content,
			ToolCalls: stream.toolCalls(),
		},
		Reasoning: stream.reasoning,
		Metrics:   metrics,
	}, nil
}

type chatRequest struct {
	Model         string           `json:"model"`
	Messages      []ChatMessage    `json:"messages"`
	Temperature   float64          `json:"temperature"`
	TopP          float64          `json:"top_p"`
	Stream        bool             `json:"stream"`
	StreamOptions streamOptions    `json:"stream_options,omitempty"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatStreamChunk struct {
	Usage   *usage       `json:"usage,omitempty"`
	Timings *timings     `json:"timings,omitempty"`
	Choices []streamChoice `json:"choices,omitempty"`
}

type streamChoice struct {
	Delta        streamDelta  `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

type streamDelta struct {
	Role           string             `json:"role,omitempty"`
	Content        string             `json:"content,omitempty"`
	Reasoning      string             `json:"reasoning_content,omitempty"`
	ToolCalls      []toolCallDelta    `json:"tool_calls,omitempty"`
}

type toolCallDelta struct {
	Index    int `json:"index"`
	ID       string `json:"id,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type timings struct {
	PromptN       *int   `json:"prompt_n,omitempty"`
	PromptMs      *int   `json:"prompt_ms,omitempty"`
	PredictedN    *int   `json:"predicted_n,omitempty"`
	PredictedMs   *int   `json:"predicted_ms,omitempty"`
	PromptEvalCount     *int `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration  *int `json:"prompt_eval_duration,omitempty"`
	EvalCount           *int `json:"eval_count,omitempty"`
	EvalDuration        *int `json:"eval_duration,omitempty"`
}

type streamState struct {
	content     string
	reasoning   string
	finishReason FinishReason
	toolCallsByIndex map[int]OpenAIToolCall
}

func readChatStream(ctx context.Context, r io.Reader, onDelta func(string)) (streamState, *usage, *timings, error) {
	state := streamState{
		finishReason:     FinishStop,
		toolCallsByIndex: make(map[int]OpenAIToolCall),
	}
	var lastUsage *usage
	var lastTimings *timings

	scanner := bufio.NewScanner(r)
	sawData := false
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return state, lastUsage, lastTimings, ctx.Err()
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		sawData = true

		var chunk chatStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			lastUsage = chunk.Usage
		}
		if chunk.Timings != nil {
			lastTimings = chunk.Timings
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		if choice.FinishReason != nil {
			state.finishReason = narrowFinishReason(*choice.FinishReason)
		}
		if choice.Delta.Content != "" {
			state.content += choice.Delta.Content
			if onDelta != nil {
				onDelta(choice.Delta.Content)
			}
		}
		if choice.Delta.Reasoning != "" {
			state.reasoning += choice.Delta.Reasoning
		}
		for _, tc := range choice.Delta.ToolCalls {
			existing := state.toolCallsByIndex[tc.Index]
			if tc.ID != "" {
				existing.ID = tc.ID
			}
			if tc.Function.Name != "" {
				existing.Function.Name = tc.Function.Name
			}
			existing.Function.Arguments += tc.Function.Arguments
			existing.Type = "function"
			state.toolCallsByIndex[tc.Index] = existing
		}
	}
	if err := scanner.Err(); err != nil {
		return state, lastUsage, lastTimings, fmt.Errorf("read stream: %w", err)
	}
	if !sawData {
		return state, lastUsage, lastTimings, fmt.Errorf("no SSE data received")
	}

	for i := 0; i < len(state.toolCallsByIndex); i++ {
		if tc, ok := state.toolCallsByIndex[i]; ok {
			if tc.ID == "" {
				tc.ID = fmt.Sprintf("call_%d", i)
			}
			if tc.Function.Arguments == "" {
				tc.Function.Arguments = "{}"
			}
			state.toolCallsByIndex[i] = tc
		}
	}
	return state, lastUsage, lastTimings, nil
}

func (s *streamState) toolCalls() []OpenAIToolCall {
	var keys []int
	for k := range s.toolCallsByIndex {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var calls []OpenAIToolCall
	for _, k := range keys {
		calls = append(calls, s.toolCallsByIndex[k])
	}
	return calls
}

func narrowFinishReason(value string) FinishReason {
	switch value {
	case "stop":
		return FinishStop
	case "tool_calls", "function_call":
		return FinishToolCalls
	case "length":
		return FinishLength
	case "content_filter":
		return FinishContentFilter
	default:
		return FinishStop
	}
}

func normalizeEndpoint(endpoint string) string {
	endpoint = strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(endpoint, "/v1/chat/completions") {
		return endpoint
	}
	return endpoint + "/v1/chat/completions"
}

func extractMetrics(modelID string, u *usage, t *timings, requestTimeMs int64) model.ModelMetrics {
	var promptEvalTokens, promptEvalTimeMs int
	var completionEvalTokens, completionEvalTimeMs int
	if t != nil {
		if t.PromptN != nil {
			promptEvalTokens = *t.PromptN
		}
		if t.PromptMs != nil {
			promptEvalTimeMs = *t.PromptMs
		}
		if t.PredictedN != nil {
			completionEvalTokens = *t.PredictedN
		}
		if t.PredictedMs != nil {
			completionEvalTimeMs = *t.PredictedMs
		}
		if promptEvalTokens == 0 && t.PromptEvalCount != nil {
			promptEvalTokens = *t.PromptEvalCount
		}
		if promptEvalTimeMs == 0 && t.PromptEvalDuration != nil {
			promptEvalTimeMs = *t.PromptEvalDuration
		}
		if completionEvalTokens == 0 && t.EvalCount != nil {
			completionEvalTokens = *t.EvalCount
		}
		if completionEvalTimeMs == 0 && t.EvalDuration != nil {
			completionEvalTimeMs = *t.EvalDuration
		}
	}

	promptTokens := promptEvalTokens
	completionTokens := completionEvalTokens
	totalTokens := promptTokens + completionTokens
	if u != nil {
		if u.PromptTokens != 0 {
			promptTokens = u.PromptTokens
		}
		if u.CompletionTokens != 0 {
			completionTokens = u.CompletionTokens
		}
		if u.TotalTokens != 0 {
			totalTokens = u.TotalTokens
		} else {
			totalTokens = promptTokens + completionTokens
		}
	}

	metrics := model.ModelMetrics{
		Model:              modelID,
		RequestCount:       1,
		PromptTokens:       promptTokens,
		CompletionTokens:   completionTokens,
		TotalTokens:        totalTokens,
		TotalRequestTimeMs: requestTimeMs,
		Requests: []model.RequestMetrics{
			{PromptTokens: promptTokens, CompletionTokens: completionTokens, RequestTimeMs: requestTimeMs},
		},
	}
	if promptEvalTokens != 0 || promptEvalTimeMs != 0 {
		metrics.PromptEvalTokens = promptEvalTokens
		metrics.PromptEvalTimeMs = int64(promptEvalTimeMs)
	}
	if completionEvalTokens != 0 || completionEvalTimeMs != 0 {
		metrics.CompletionEvalTokens = completionEvalTokens
		metrics.CompletionEvalTimeMs = int64(completionEvalTimeMs)
	}
	return metrics
}
