export interface ScenarioInfo {
  id: string;
  name: string;
  category: string;
  difficulty: string;
  maxPoints: number;
  prompt: string;
  track: string;
}

export interface ModelsResponse {
  local: Model[];
  remote: Model[];
}

export interface Model {
  id: string;
  source: "local" | "remote";
  endpoint: string;
  requiresApiKey?: boolean;
  displayName?: string;
}

export interface RunSummary {
  id: string;
  startedAt: number;
  finishedAt?: number;
  status: string;
  model: string;
  totalPoints: number;
  maxPoints: number;
}

export interface RunDetail {
  id: string;
  startedAt: number;
  finishedAt?: number;
  status: string;
  model: string;
  totalPoints: number;
  maxPoints: number;
  scenarios: ScenarioResult[];
}

export interface ScenarioResult {
  scenarioId: string;
  category?: string;
  family?: string;
  status: "pass" | "partial" | "fail" | "stopped" | "skipped";
  points: number;
  maxPoints: number;
  wallTimeMs?: number;
  firstTokenMs?: number;
  toolCallCount: number;
  modelMetrics?: ModelMetrics;
  evaluation?: ScenarioEvaluation;
  rubricKind?: string;
  breakdown?: {
    correctness: number;
    scope: number;
    pattern: number;
    verification: number;
    cleanup: number;
  };
  error?: string;
}

export interface ReportData {
  models: unknown[];
  categories: unknown[];
  difficulty: unknown[];
}

export interface RuntimeConfig {
  localEndpoint: string;
  remoteEndpoint: string;
  remoteApiKey: string;
  remoteModels: string[];
  remoteModelCacheTTLSeconds: number;
}

export interface OneshotTestSummary {
  id: string;
  title: string;
  category: string;
  prompt: string;
}

export interface OneshotResult {
  promptId: string;
  runId: string;
  model?: string;
  startedAt?: number;
  finishedAt?: number;
  status: "pending" | "running" | "done" | "failed" | "stopped";
  output?: string;
  finishReason?: string;
  wallTimeMs?: number;
  firstTokenMs?: number;
  promptTokens?: number;
  completionTokens?: number;
  artifactPath?: string;
  error?: string;
  hasArtifact: boolean;
}

export interface OneshotLatestRun {
  runId: string;
  status: string;
  model?: string;
  endpoint?: string;
  promptIds: string[];
  startedAt: number;
  finishedAt?: number;
  error?: string;
  results: OneshotResult[];
}

export type PersistedEventBase = { seq: number; ts: number };

export type ToolCall = { name: string; args: string; turn: number; result?: unknown };

export type ModelMetrics = {
  model?: string;
  requestCount: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  totalRequestTimeMs: number;
  promptEvalTokens?: number;
  promptEvalTimeMs?: number;
  completionEvalTokens?: number;
  completionEvalTimeMs?: number;
};

export type EvaluationCheck = { name: string; pass: boolean; detail?: string };

export type ScenarioEvaluation = {
  status: "pass" | "partial" | "fail";
  points: number;
  maxPoints: number;
  checks: EvaluationCheck[];
  summary: string;
  rubricKind?: string;
  breakdown?: {
    correctness: number;
    scope: number;
    pattern: number;
    verification: number;
    cleanup: number;
  };
};

export type PersistedEvent =
  | (PersistedEventBase & {
      type: "run_started";
      runId: string;
      scenarioIds: string[];
      model?: string | null;
      endpoint?: string | null;
      harness?: string | null;
    })
  | (PersistedEventBase & {
      type: "model_warmup_started";
      runId: string;
      model: string;
      endpoint: string;
    })
  | (PersistedEventBase & {
      type: "model_warmup_finished";
      runId: string;
      durationMs: number;
      modelFile?: string;
      quant?: string;
      gpuBackend?: string;
      gpuModel?: string;
    })
  | (PersistedEventBase & {
      type: "run_finished";
      runId: string;
      totalPoints: number;
      maxPoints: number;
      reportPath?: string | null;
    })
  | (PersistedEventBase & { type: "run_stopped"; runId: string; reason?: string })
  | (PersistedEventBase & { type: "run_failed"; runId: string; error: string })
  | (PersistedEventBase & {
      type: "scenario_started";
      runId: string;
      scenarioId: string;
      name?: string;
      category: string;
      maxPoints: number;
      family?: string;
      rubricKind?: string;
    })
  | (PersistedEventBase & {
      type: "scenario_finished";
      runId: string;
      scenarioId: string;
      status: "pass" | "partial" | "fail" | "stopped";
      points: number;
      wallTimeMs: number;
      toolCallCount: number;
      firstTokenMs?: number;
      turnWallTimes?: number[];
      turnFirstTokenMs?: number[];
      evaluation: ScenarioEvaluation;
      modelMetrics?: ModelMetrics;
      errorKind?: "infra" | "timeout" | "aborted" | "runtime";
      family?: string;
      rubricKind?: string;
      rubricBreakdown?: {
        correctness: number;
        scope: number;
        pattern: number;
        verification: number;
        cleanup: number;
      } | null;
    })
  | (PersistedEventBase & { type: "assistant"; runId: string; scenarioId: string; content: string })
  | (PersistedEventBase & {
      type: "assistant_delta";
      runId: string;
      scenarioId: string;
      content: string;
    })
  | (PersistedEventBase & { type: "tool_call"; runId: string; scenarioId: string; call: ToolCall })
  | (PersistedEventBase & {
      type: "tool_result";
      runId: string;
      scenarioId: string;
      call: ToolCall;
      result: string;
    })
  | (PersistedEventBase & {
      type: "model_metrics";
      runId: string;
      scenarioId: string;
      metrics: ModelMetrics;
    });

export type LogEntryKind = "assistant" | "tool" | "stdout" | "stderr" | "system";

export type LogEntry = {
  id: number;
  kind: LogEntryKind;
  label: string;
  text: string;
  time: string;
};

export type ScenarioStatus = "pending" | "running" | "pass" | "partial" | "fail" | "stopped";

export type ScenarioState = {
  id: string;
  name: string;
  category: string;
  maxPoints: number;
  status: ScenarioStatus;
  startedAt?: number;
  finishedAt?: number;
  points?: number;
  wallTimeMs?: number;
  toolCallCount?: number;
  bashCallCount?: number;
  editCallCount?: number;
  firstTokenMs?: number;
  turnWallTimes?: number[];
  turnFirstTokenMs?: number[];
  logs: LogEntry[];
  streamBuffer: string;
  liveMetrics?: ModelMetrics;
  evaluation?: ScenarioEvaluation;
};

export type RunStatus = "idle" | "warming_up" | "running" | "done" | "stopped" | "failed";

export type RunState = {
  runId: string | null;
  status: RunStatus;
  startedAt?: number;
  scenarios: ScenarioState[];
  activeScenarioId: string | null;
  focusedScenarioId: string | null;
  totalPoints: number;
  maxPoints: number;
  globalMetrics?: ModelMetrics;
  model?: string | null;
};

export type BackendEvent = {
  seq: number;
  ts: number;
  type: string;
  payload: Record<string, unknown>;
  runId: string;
  scenarioId?: string;
};

export function normalizeBackendEvent(ev: BackendEvent): PersistedEvent | null {
  const base = { seq: ev.seq, ts: ev.ts, runId: ev.runId, scenarioId: ev.scenarioId ?? "" };
  switch (ev.type) {
    case "run_started": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "run_started",
        scenarioIds: Array.isArray(p.scenarioIds) ? (p.scenarioIds as string[]) : [],
        model: (p.model as string | null) ?? null,
        endpoint: (p.endpoint as string | null) ?? null,
        harness: (p.harness as string | null) ?? null,
      };
    }
    case "model_warmup_started": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "model_warmup_started",
        model: (p.model as string) ?? "",
        endpoint: (p.endpoint as string) ?? "",
      };
    }
    case "model_warmup_finished": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "model_warmup_finished",
        durationMs: typeof p.durationMs === "number" ? p.durationMs : 0,
        modelFile: (p.modelFile as string) ?? undefined,
        quant: (p.quant as string) ?? undefined,
        gpuBackend: (p.gpuBackend as string) ?? undefined,
        gpuModel: (p.gpuModel as string) ?? undefined,
      };
    }
    case "run_finished": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "run_finished",
        totalPoints: typeof p.totalPoints === "number" ? p.totalPoints : 0,
        maxPoints: typeof p.maxPoints === "number" ? p.maxPoints : 0,
        reportPath: (p.reportPath as string | null) ?? null,
      };
    }
    case "run_stopped":
      return {
        ...base,
        type: "run_stopped",
        reason: ((ev.payload as Record<string, unknown>)?.reason as string) ?? undefined,
      };
    case "run_failed":
      return {
        ...base,
        type: "run_failed",
        error: ((ev.payload as Record<string, unknown>)?.error as string) ?? "",
      };
    case "scenario_started": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "scenario_started",
        scenarioId: (p.scenarioId as string) ?? base.scenarioId,
        name: (p.name as string) ?? undefined,
        category: (p.category as string) ?? "",
        maxPoints: typeof p.maxPoints === "number" ? p.maxPoints : 0,
        family: (p.family as string) ?? undefined,
        rubricKind: (p.rubricKind as string) ?? undefined,
      };
    }
    case "scenario_finished": {
      const p = (ev.payload ?? {}) as Record<string, unknown>;
      return {
        ...base,
        type: "scenario_finished",
        scenarioId: (p.scenarioId as string) ?? base.scenarioId,
        status: (p.status as "pass" | "partial" | "fail" | "stopped") ?? "fail",
        points: typeof p.points === "number" ? p.points : 0,
        wallTimeMs: typeof p.wallTimeMs === "number" ? p.wallTimeMs : 0,
        toolCallCount: typeof p.toolCallCount === "number" ? p.toolCallCount : 0,
        firstTokenMs: typeof p.firstTokenMs === "number" ? p.firstTokenMs : undefined,
        turnWallTimes: Array.isArray(p.turnWallTimes) ? p.turnWallTimes : undefined,
        turnFirstTokenMs: Array.isArray(p.turnFirstTokenMs) ? p.turnFirstTokenMs : undefined,
        evaluation: (p.evaluation as ScenarioEvaluation) ?? { status: "fail", points: 0, maxPoints: 0, checks: [], summary: "" },
        modelMetrics: (p.modelMetrics as ModelMetrics) ?? undefined,
        family: (p.family as string) ?? undefined,
        rubricKind: (p.rubricKind as string) ?? undefined,
        rubricBreakdown: (p.rubricBreakdown as any) ?? undefined,
      };
    }
    case "assistant":
      return {
        ...base,
        type: "assistant",
        content: ((ev.payload as Record<string, unknown>)?.content as string) ?? "",
      };
    case "assistant_delta":
      return {
        ...base,
        type: "assistant_delta",
        content: ((ev.payload as Record<string, unknown>)?.content as string) ?? "",
      };
    case "tool_call":
      return {
        ...base,
        type: "tool_call",
        call: (ev.payload as Record<string, unknown>)?.call as ToolCall,
      };
    case "tool_result":
      return {
        ...base,
        type: "tool_result",
        call: ((ev.payload as Record<string, unknown>)?.call as ToolCall) ?? { name: "", args: "", turn: 0 },
        result: ((ev.payload as Record<string, unknown>)?.result as string) ?? "",
      };
    case "model_metrics":
      return {
        ...base,
        type: "model_metrics",
        metrics: ((ev.payload as Record<string, unknown>)?.metrics as ModelMetrics) ?? {
          requestCount: 0,
          promptTokens: 0,
          completionTokens: 0,
          totalTokens: 0,
          totalRequestTimeMs: 0,
        },
      };
    default:
      return null;
  }
}
