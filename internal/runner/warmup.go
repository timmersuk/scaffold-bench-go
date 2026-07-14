package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// WarmupResult contains the metadata extracted from the warmup response.
type WarmupResult struct {
	ModelFile   string
	Quant       string
	QuantTier   *float64
	QuantSource string
	ContextSize *int
	GPUBackend  string
	GPUModel    string
	GPUCount    *int
	VRAMTotalMB *int
}

// WarmupResponse represents the response from a completion request.
type WarmupResponse struct {
	Model   string `json:"model"`
	Usage   *Usage `json:"usage,omitempty"`
	Timings *struct {
		PromptN      int `json:"prompt_n"`
		PromptMS     int `json:"prompt_ms"`
		PredictedN   int `json:"predicted_n"`
		PredictedMS  int `json:"predicted_ms"`
	} `json:"timings,omitempty"`
}

// Usage represents token usage in a completion response.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ModelsResponse represents the response from /v1/models.
type ModelsResponse struct {
	Data []ModelInfo `json:"data"`
}

// ModelInfo represents a single model in the models list.
type ModelInfo struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
	ContextSize *int   `json:"context_size,omitempty"`
	MaxModelLen *int   `json:"max_model_len,omitempty"`
}

// performWarmup sends a minimal completion request to warm up the model and extract metadata.
func performWarmup(ctx context.Context, endpoint, modelID, apiKey string, timeout time.Duration) (*WarmupResult, error) {
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	reqBody := map[string]any{
		"model":       modelID,
		"messages":    []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens":  1,
		"temperature": 0,
		"stream":      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal warmup request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create warmup request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("warmup request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("warmup request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var warmupResp WarmupResponse
	if err := json.NewDecoder(resp.Body).Decode(&warmupResp); err != nil {
		return nil, fmt.Errorf("decode warmup response: %w", err)
	}

	result := &WarmupResult{}

	// Extract metadata from the response
	extractGPUInfo(resp, result)
	extractQuantInfo(warmupResp.Model, result)

	// Try to get context size from /v1/models
	fetchModelMetadata(ctx, endpoint, modelID, apiKey, result)

	return result, nil
}

// extractGPUInfo extracts GPU information from response headers.
func extractGPUInfo(resp *http.Response, result *WarmupResult) {
	// llama.cpp server may include GPU info in headers
	if backend := resp.Header.Get("X-Backend"); backend != "" {
		result.GPUBackend = backend
	}
	if gpuModel := resp.Header.Get("X-GPU-Model"); gpuModel != "" {
		result.GPUModel = gpuModel
	}
	if gpuCount := resp.Header.Get("X-GPU-Count"); gpuCount != "" {
		if count, err := strconv.Atoi(gpuCount); err == nil {
			result.GPUCount = &count
		}
	}
	if vram := resp.Header.Get("X-VRAM-MB"); vram != "" {
		if vramMB, err := strconv.Atoi(vram); err == nil {
			result.VRAMTotalMB = &vramMB
		}
	}
}

// quantPattern matches quantization tags like Q4_K_M, Q8_0, etc.
var quantPattern = regexp.MustCompile(`(?i)(Q\d+[_A-Z]*)`)

// quantTierPattern extracts the numeric tier from a quant tag.
var quantTierPattern = regexp.MustCompile(`(?i)Q(\d+)`)

// extractQuantInfo extracts quantization info from a model identifier.
func extractQuantInfo(modelID string, result *WarmupResult) {
	if modelID == "" {
		return
	}

	// Extract quant tag
	quantMatch := quantPattern.FindString(modelID)
	if quantMatch != "" {
		result.Quant = strings.ToUpper(quantMatch)

		// Extract numeric tier
		tierMatch := quantTierPattern.FindStringSubmatch(quantMatch)
		if len(tierMatch) > 1 {
			if tier, err := strconv.ParseFloat(tierMatch[1], 64); err == nil {
				result.QuantTier = &tier
			}
		}
	}

	// Extract quant source (uploader name from path like "TheBloke/...")
	parts := strings.Split(modelID, "/")
	if len(parts) >= 2 {
		result.QuantSource = parts[0]
	}

	// Use modelID as model file if no file path is available
	result.ModelFile = modelID
}

// fetchModelMetadata fetches additional metadata from /v1/models endpoint.
func fetchModelMetadata(ctx context.Context, endpoint, modelID, apiKey string, result *WarmupResult) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/v1/models", nil)
	if err != nil {
		return
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var modelsResp ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return
	}

	// Find the model in the list
	for _, m := range modelsResp.Data {
		if m.ID == modelID || strings.HasSuffix(m.ID, "/"+modelID) {
			if m.ContextSize != nil {
				result.ContextSize = m.ContextSize
			} else if m.MaxModelLen != nil {
				result.ContextSize = m.MaxModelLen
			}
			break
		}
	}
}
