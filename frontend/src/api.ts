import type {
  ModelsResponse,
  OneshotLatestRun,
  OneshotTestSummary,
  ReportData,
  RunSummary,
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

export const api = {
  getScenarios: (signal?: AbortSignal) => get<ScenarioInfo[]>("/scenarios", signal),
  getModels: (signal?: AbortSignal) => get<ModelsResponse>("/models", signal),
  listRuns: (signal?: AbortSignal) => get<RunSummary[]>("/runs", signal),
  activeRun: (signal?: AbortSignal) => get<{ runId: string | null }>("/runs/active", signal),
  createRun: (body: { scenarioIds: string[]; modelId?: string }) =>
    post<{ runId: string }>("/runs", body),
  stopRun: (id: string) => post<{ ok: boolean }>(`/runs/${id}/stop`),
  clearRuns: () => post<{ ok: boolean }>("/runs/clear"),
  getReportData: (signal?: AbortSignal) => get<ReportData>("/report/data", signal),
  oneshotTests: (signal?: AbortSignal) => get<OneshotTestSummary[]>("/oneshot/tests", signal),
  startOneshot: (body: { modelId: string; promptIds: string[] }) =>
    post<{ runId: string }>("/oneshot/runs", body),
  latestOneshot: (signal?: AbortSignal) =>
    get<OneshotLatestRun | null>("/oneshot/runs/latest", signal),
};
