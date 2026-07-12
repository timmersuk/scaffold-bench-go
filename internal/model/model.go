package model

// Event is a persisted run or scenario event streamed to clients.
type Event struct {
	Seq        int64  `json:"seq"`
	Ts         int64  `json:"ts"`
	Type       string `json:"type"`
	Payload    any    `json:"payload"`
	RunID      string `json:"runId"`
	ScenarioID string `json:"scenarioId,omitempty"`
}

// RunStatus is the lifecycle status of a benchmark run.
type RunStatus string

const (
	RunRunning RunStatus = "running"
	RunDone    RunStatus = "done"
	RunFailed  RunStatus = "failed"
	RunStopped RunStatus = "stopped"
)

// ScenarioStatus is the result status of an individual scenario run.
type ScenarioStatus string

const (
	ScenarioPending ScenarioStatus = "pending"
	ScenarioRunning ScenarioStatus = "running"
	ScenarioPass    ScenarioStatus = "pass"
	ScenarioPartial ScenarioStatus = "partial"
	ScenarioFail    ScenarioStatus = "fail"
	ScenarioStopped ScenarioStatus = "stopped"
)

// Event types produced by the run engine.
const (
	EventRunStarted      = "run_started"
	EventScenarioStarted = "scenario_started"
	EventAssistant       = "assistant"
	EventAssistantDelta  = "assistant_delta"
	EventToolCall        = "tool_call"
	EventToolResult      = "tool_result"
	EventModelMetrics    = "model_metrics"
	EventScenarioFinished= "scenario_finished"
	EventRunFinished     = "run_finished"
	EventRunFailed       = "run_failed"
	EventRunStopped      = "run_stopped"
)

// Run is a top-level benchmark run.
type Run struct {
	ID              string   `json:"id"`
	StartedAt       int64    `json:"startedAt"`
	FinishedAt      *int64   `json:"finishedAt,omitempty"`
	Status          RunStatus `json:"status"`
	ScenarioIDs     []string `json:"scenarioIds"`
	Runtime         string   `json:"runtime"`
	RuntimeKind     string   `json:"runtimeKind"`
	Endpoint        string   `json:"endpoint,omitempty"`
	Model           string   `json:"model"`
	ModelFile       string   `json:"modelFile,omitempty"`
	Quant           string   `json:"quant,omitempty"`
	QuantTier       *float64 `json:"quantTier,omitempty"`
	QuantSource     string   `json:"quantSource,omitempty"`
	ContextSize     *int     `json:"contextSize,omitempty"`
	Harness         string   `json:"harness,omitempty"`
	GPUBackend      string   `json:"gpuBackend,omitempty"`
	GPUModel        string   `json:"gpuModel,omitempty"`
	GPUCount        *int     `json:"gpuCount,omitempty"`
	VRAMTotalMB     *int     `json:"vramTotalMB,omitempty"`
	HostThermalNote string   `json:"hostThermalNote,omitempty"`
	TotalPoints     *int     `json:"totalPoints,omitempty"`
	MaxPoints       *int     `json:"maxPoints,omitempty"`
	ReportPath      string   `json:"reportPath,omitempty"`
	Error           string   `json:"error,omitempty"`
}

// ScenarioRun is the result of running a single scenario within a run.
type ScenarioRun struct {
	RunID           string         `json:"runId"`
	ScenarioID      string         `json:"scenarioId"`
	Category        string         `json:"category,omitempty"`
	Family          string         `json:"family,omitempty"`
	StartedAt       *int64         `json:"startedAt,omitempty"`
	FinishedAt      *int64         `json:"finishedAt,omitempty"`
	Status          ScenarioStatus `json:"status"`
	Points          *int           `json:"points,omitempty"`
	MaxPoints       int            `json:"maxPoints"`
	RubricKind      string         `json:"rubricKind,omitempty"`
	Correctness     *int           `json:"correctness,omitempty"`
	Scope           *int           `json:"scope,omitempty"`
	Pattern         *int           `json:"pattern,omitempty"`
	Verification    *int           `json:"verification,omitempty"`
	Cleanup         *int           `json:"cleanup,omitempty"`
	WallTimeMs      *int64         `json:"wallTimeMs,omitempty"`
	FirstTokenMs    *int64         `json:"firstTokenMs,omitempty"`
	ToolCallCount   *int           `json:"toolCallCount,omitempty"`
	BashCalls       *int           `json:"bashCalls,omitempty"`
	PostChangeBashCalls *int       `json:"postChangeBashCalls,omitempty"`
	VerifyPasses    *int           `json:"verifyPasses,omitempty"`
	Mutated         *bool          `json:"mutated,omitempty"`
	ModelMetricsJSON string        `json:"modelMetricsJSON,omitempty"`
	EvaluationJSON  string         `json:"evaluationJSON,omitempty"`
	ErrorKind       string         `json:"errorKind,omitempty"`
	Error           string         `json:"error,omitempty"`
	ArtifactPath    string         `json:"artifactPath,omitempty"`
}

// Evaluation is the scoring result for a scenario run.
type Evaluation struct {
	Status      string        `json:"status"`
	Points      int           `json:"points"`
	MaxPoints   int           `json:"maxPoints"`
	Checks      []CheckResult `json:"checks"`
	Summary     string        `json:"summary"`
	RubricKind  string        `json:"rubricKind,omitempty"`
	Breakdown   Breakdown     `json:"breakdown,omitempty"`
}

// Breakdown is the per-rubric-axis score.
type Breakdown struct {
	Correctness  int `json:"correctness"`
	Scope        int `json:"scope"`
	Pattern      int `json:"pattern"`
	Verification int `json:"verification"`
	Cleanup      int `json:"cleanup"`
}

// CheckResult is the outcome of a single rubric check.
type CheckResult struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Weight int    `json:"weight"`
	Detail string `json:"detail,omitempty"`
}

// ToolCall is a single tool invocation made by the agent.
type ToolCall struct {
	Name   string `json:"name"`
	Args   string `json:"args"`
	Turn   int    `json:"turn"`
	Result string `json:"result,omitempty"`
}

// ModelMetrics aggregates token and timing data across model calls.
type ModelMetrics struct {
	Model              string           `json:"model"`
	RequestCount       int              `json:"requestCount"`
	PromptTokens       int              `json:"promptTokens"`
	CompletionTokens   int              `json:"completionTokens"`
	TotalTokens        int              `json:"totalTokens"`
	TotalRequestTimeMs int64            `json:"totalRequestTimeMs"`
	PromptEvalTokens   int              `json:"promptEvalTokens,omitempty"`
	PromptEvalTimeMs   int64            `json:"promptEvalTimeMs,omitempty"`
	CompletionEvalTokens int            `json:"completionEvalTokens,omitempty"`
	CompletionEvalTimeMs int64          `json:"completionEvalTimeMs,omitempty"`
	Requests           []RequestMetrics `json:"requests,omitempty"`
}

// RequestMetrics is a single model call data point.
type RequestMetrics struct {
	PromptTokens     int   `json:"promptTokens"`
	CompletionTokens int   `json:"completionTokens"`
	RequestTimeMs    int64 `json:"requestTimeMs"`
}

// RuntimeEvent is an event emitted by the agent runtime.
type RuntimeEvent struct {
	Type    string       `json:"type"`
	Delta   string       `json:"delta,omitempty"`
	Content string       `json:"content,omitempty"`
	Call    *ToolCall    `json:"call,omitempty"`
	Result  string       `json:"result,omitempty"`
	Metrics *ModelMetrics `json:"metrics,omitempty"`
}

// WorkspaceArchive captures the diff between the mutated workspace and its pristine copy.
type WorkspaceArchive struct {
	Version int                     `json:"version"`
	Changed []WorkspaceArchiveEntry `json:"changed"`
	Deleted []string                `json:"deleted"`
}

// WorkspaceArchiveEntry is a changed file in the archive.
type WorkspaceArchiveEntry struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
