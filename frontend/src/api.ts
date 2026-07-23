import type {
  BackendEvent,
  BatchRun,
  BatchRunDetail,
  ModelsResponse,
  OneshotLatestRun,
  OneshotTestSummary,
  ReportData,
  RunDetail,
  RunSummary,
  RuntimeConfig,
  ScenarioInfo,
} from "./types";

const BASE = "/api";

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { signal });
  if (!res.ok) throw new ApiError(`GET ${path} -> ${res.status}`, res.status);
  return res.json() as Promise<T>;
}

async function post<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: body ? { "content-type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw new ApiError(`POST ${path} -> ${res.status}`, res.status);
  return res.json() as Promise<T>;
}

async function put<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "PUT",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new ApiError(`PUT ${path} -> ${res.status}`, res.status);
  return res.json() as Promise<T>;
}

export const api = {
  getScenarios: (signal?: AbortSignal) => get<ScenarioInfo[]>("/scenarios", signal),
  getModels: (signal?: AbortSignal) => get<ModelsResponse>("/models", signal),
  listRuns: (signal?: AbortSignal) => get<RunSummary[]>("/runs", signal),
  getRun: (id: string, signal?: AbortSignal) => get<RunDetail>(`/runs/${id}`, signal),
  activeRun: (signal?: AbortSignal) => get<{ runId: string | null }>("/runs/active", signal),
  createRun: (body: {
    scenarioIds: string[];
    modelId?: string;
    systemPrompt?: string;
    timeoutMs?: number;
  }) => post<{ runId: string }>("/runs", body),
  stopRun: (id: string) => post<{ ok: boolean }>(`/runs/${id}/stop`),
  clearRuns: () => post<{ ok: boolean }>("/runs/clear"),
  getRunEvents: (runId: string, fromSeq = -1, signal?: AbortSignal) =>
    get<BackendEvent[]>(`/runs/${runId}/events?fromSeq=${fromSeq}`, signal),
  getReportData: (signal?: AbortSignal) => get<ReportData>("/report/data", signal),
  oneshotTests: (signal?: AbortSignal) => get<OneshotTestSummary[]>("/oneshot/tests", signal),
  startOneshot: (body: { modelId: string; promptIds: string[] }) =>
    post<{ runId: string }>("/oneshot/runs", body),
  stopOneshot: (id: string) => post<{ ok: boolean; runId: string; status: string }>(`/oneshot/runs/${id}/stop`),
  latestOneshot: (signal?: AbortSignal) =>
    get<OneshotLatestRun | null>("/oneshot/runs/latest", signal),
  oneshotArtifactUrl: (promptId: string, version?: number) =>
    `${BASE}/oneshot/artifacts/${promptId}${version ? `?v=${version}` : ""}`,
  getConfig: (signal?: AbortSignal) => get<RuntimeConfig>("/config", signal),
  updateConfig: (body: RuntimeConfig) => put<RuntimeConfig>("/config", body),
  listBatchRuns: (signal?: AbortSignal) => get<BatchRun[]>("/batch-runs", signal),
  getBatchRun: (id: string, signal?: AbortSignal) => get<BatchRunDetail>(`/batch-runs/${id}`, signal),
  activeBatchRun: (signal?: AbortSignal) => get<{ batchId: string | null }>("/batch-runs/active", signal),
  createBatchRun: (body: {
    modelIds: string[];
    scenarioIds: string[];
    runsPerModel: number;
    warmupDuration: number;
    harness: string;
  }) => post<{ batchId: string }>("/batch-runs", body),
  stopBatchRun: (id: string) => post<{ ok: boolean }>(`/batch-runs/${id}/stop`),
};
