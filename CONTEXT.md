# Scaffold Bench Context

Language for the Go-flavored scaffold-bench scenario format: neutral YAML manifests, fixture files, and evaluator-driven scoring.

## Language

**Scenario**:
A single coding task handed to a model, scored against a 10-point (or custom) rubric.
_Avoid_: Exercise, problem.

**Scenario Port**:
A translated version of an upstream TypeScript scenario from `1337hero/scaffold-bench` into this repo’s manifest/fixture format.

**Manifest**:
The hand-editable YAML file that declares a scenario’s metadata, workspace, rubric, and labels. The Go evaluator loads it at runtime.

**Fixture**:
A file shipped with a scenario: the starter workspace tree, the pristine reference copy, or a hidden behavioral test.

**Workspace Source**:
The starter files placed in the model’s work directory at the start of a run. For SB-01 this is `scenarios/SB-01/playground/`.

**Pristine Copy**:
The clean, un-mutated snapshot of the workspace used for diff-based scope and pattern checks. It is the reference for `function_equals_original`, `no_added_comments`, and `files_changed_only`.
_Avoid_: Golden workspace.

**Signal Type**:
The dominant correctness signal used by a scenario. It is metadata only; the evaluator still runs every configured rubric check.

**Behavioral Source of Truth**:
The behavior defined by the original TypeScript scenario in `1337hero/scaffold-bench`. When porting, the upstream fixtures and hidden tests are copied verbatim unless an explicit compatibility issue is declared.

**Upstream Provenance**:
A record that a field or fixture came from the original TypeScript scenario, kept for audit but not used by the Go evaluator.
_Avoid_: Source of truth.

**AST Check**:
A rubric check whose `type` begins with `ast_`, implemented as a native Go check that analyzes TypeScript or TSX source files to match the behavior of the upstream TypeScript benchmark. AST checks do not shell out; they are ordinary evaluator checks.

**Behavioral Source of Truth**:
The behavior defined by the original TypeScript scenario in `1337hero/scaffold-bench`. When porting, the upstream fixtures and hidden tests are copied verbatim unless an explicit compatibility issue is declared.

**Hidden Test**:
A test fixture kept outside the model workspace and copied into a temporary directory only during evaluation so the model cannot read the assertions.

**Model**:
A selectable Large Language Model available through an inference endpoint. In the API it is represented by an `id`, `source` (`local` | `remote`), `endpoint`, and an optional `requiresApiKey` flag.
_Avoid_: LLM, inference target.

## Benchmark Run

**Run**:
A single benchmark execution against one or more selected scenarios using one Model.

**Active Run**:
A Run that the server is currently executing. Only one Active Run is tracked at a time; it is discoverable via `/api/runs/active`.

**Run Stream**:
The server-sent event feed that emits a Run's events in real time, available at `/api/runs/{id}/stream`.

**Run Event**:
A persisted, sequenced message emitted by the runner during a Run. Examples include `run_started`, `scenario_started`, `assistant_delta`, and `scenario_finished`.

**Runtime Configuration**:
Configuration persisted in the data folder and editable at runtime, restricted to model endpoints (`LocalEndpoint`, `RemoteEndpoint`, `RemoteAPIKey`) and the remote model list (`RemoteModels`). Changes are blocked while a Run is **Active**; `HTTPAddr`, `DBPath`, and `DataDir` remain environment-only configuration.
_Avoid_: Settings, preferences.
The outcome of an individual Scenario within a Run, including its status (`pass`, `partial`, `fail`, `stopped`, `skipped`), points, and evaluation details.

**Dashboard**:
The primary frontend view for selecting scenarios, configuring a Model, starting a Run, and watching the Run Stream.

## Run Preflight

**Readiness Gate**:
A blocking check performed before a Run begins. It sends a minimal completion request to the Model endpoint to confirm the model is reachable and loaded into VRAM. If the gate fails, the Run is not started.
_Avoid_: Preflight check, health check.

**Metadata Survey**:
A best-effort extraction of hardware and model metadata (GPU backend, GPU model, GPU count, VRAM, quantization, context size) from the readiness gate's warmup response. If extraction fails, fields are left null and the Run proceeds.
_Avoid_: System info, hardware detection.

**Warmup Phase**:
A distinct Run phase (`warming_up` status) between run start and the first scenario. The readiness gate and metadata survey execute during this phase. It prevents model load time from contaminating scenario timing metrics.
_Avoid_: Initialization, loading phase.

## One-Shot Lab

**LabPrompt**:
A creative coding challenge defined as a markdown file with frontmatter (`title`, `category`), used in the One-Shot Lab. Produces a single HTML artifact; not scored.
_Avoid_: Test, prompt, challenge.

**LabPrompt Result**:
The outcome of executing a LabPrompt against a Model. Stores the raw output, extracted HTML artifact, token counts, and timing metrics. Uses latest-per-prompt semantics: rerunning a LabPrompt replaces its previous result without affecting other prompts' results.
_Avoid_: Test result, output.

**One-Shot Run**:
An execution of one or more LabPrompts against a single Model. Unscored, single-turn, no tools or agent loop. Produces HTML artifacts rendered in the browser.
_Avoid_: One-shot test, vibe check.

## Agent Execution

**Tool Execution Mode**:
A per-Run setting (`sequential` or `parallel`) that controls how the agent executes batches of tool calls. In `sequential` mode, all calls run one at a time in order. In `parallel` mode, Parallel-Safe Tools run concurrently while mutating tools run sequentially, preserving the model's intended ordering.
_Avoid_: Concurrency mode, execution strategy.

**Parallel-Safe Tool**:
A tool that can be executed concurrently without side effects or race conditions. In this codebase, only `read` and `ls` are Parallel-Safe. Mutating tools (`edit`, `write`) and arbitrary-command tools (`bash`) are excluded to prevent conflicts and maintain predictability.
_Avoid_: Read-only tool, safe tool.

**Tool Call Hook**:
A middleware function invoked before or after tool execution. `beforeToolCall` can inspect and optionally block a call; `afterToolCall` can inspect the result and optionally override it. Hooks are configured at the agent level, not the runner level.
_Avoid_: Tool middleware, execution interceptor.

## Scenario Classification

**Category**:
The task kind axis for scenarios. One of seven closed values: `surgical-edit`, `scope-discipline`, `verify-and-repair`, `implementation`, `read-only-analysis`, `responsiveness`, `long-context`. Determines the leaderboard's category breakdown columns.
_Avoid_: Task type, problem class.

**Difficulty**:
The cognitive-load axis for scenarios, orthogonal to Category. One of three closed values: `low`, `medium`, `high`. Determines the leaderboard's tier breakdown columns.
_Avoid_: Complexity, challenge level.

## Report & Leaderboard

**Report**:
The aggregated leaderboard data computed from all completed runs. Contains per-model metrics (Solve %, Discipline %, Verify %, token throughput, timing), per-category and per-tier breakdowns, context analysis, Pareto frontier, and awards.
_Avoid_: Summary, analytics.

**Solve %**:
The primary leaderboard metric. The percentage of scenarios where a model achieved full correctness (3/3 on the correctness axis), restricted to 10pt rubrics. Reported with Wilson 95% confidence interval.
_Avoid_: Pass rate, success rate.

**Discipline %**:
The process dimensions metric. The average percentage of (scope + pattern + verification + cleanup) / 7 across all scored scenarios for a model.
_Avoid_: Process score, style score.

**Verify %**:
The share of mutating scored runs in which the model ran a passing test/typecheck command after changing code. Measured from the tool-call trace (behavioral fingerprint), independent of rubric points. Null when no eligible data.
_Avoid_: Test rate, verification rate.

**Pareto Frontier**:
The set of models that are not dominated by any other model in both score and token efficiency. A model is on the frontier if no other model has both higher score and lower tokens.
_Avoid_: Efficiency boundary, optimal set.

**Context Cap Curve**:
A retrospective solve-rate curve showing how many scenarios a model would solve if its context window were capped at various sizes (8k, 16k, 32k, 64k, 128k). Computed from runs as they actually executed, not re-run capped — a lower bound on capped performance.
_Avoid_: Context sensitivity, window analysis.
