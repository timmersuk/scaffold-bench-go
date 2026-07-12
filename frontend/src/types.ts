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

export interface ReportData {
  models: unknown[];
  categories: unknown[];
  difficulty: unknown[];
}

export interface OneshotTestSummary {
  id: string;
  name: string;
}

export interface OneshotLatestRun {
  runId: string;
  status: string;
  model: string;
  results: unknown[];
}
