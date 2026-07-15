import type { BackendEvent, OneshotResult } from "../types";

export type OneshotPromptStatus = "pending" | "running" | "done" | "failed" | "stopped";

export type OneshotPromptState = {
  id: string;
  status: OneshotPromptStatus;
  output: string;
  model?: string | null;
  artifact?: boolean;
  artifactVersion?: number;
  finishReason?: string;
  wallTimeMs?: number;
  firstTokenMs?: number;
  promptTokens?: number;
  completionTokens?: number;
  error?: string;
};

export type OneshotState = {
  runId: string | null;
  status: "idle" | "warming_up" | "running" | "done" | "failed" | "stopped";
  model: string | null;
  promptIds: string[];
  prompts: Record<string, OneshotPromptState>;
  lastSeenSeq: number;
};

export type OneshotAction =
  | { type: "hydrate"; state: OneshotState }
  | { type: "start"; runId: string; model: string; promptIds: string[] }
  | { type: "event"; event: BackendEvent }
  | { type: "stop" }
  | { type: "reset" };

export function oneshotStateReducer(state: OneshotState, action: OneshotAction): OneshotState {
  switch (action.type) {
    case "hydrate":
      return action.state;

    case "start":
      return {
        runId: action.runId,
        status: "running",
        model: action.model,
        promptIds: action.promptIds,
        prompts: Object.fromEntries(
          action.promptIds.map((id) => [id, { id, status: "pending", output: "" }])
        ),
        lastSeenSeq: -1,
      };

    case "event": {
      const ev = action.event;
      if (ev.seq <= state.lastSeenSeq) return state;

      const p = (ev.payload ?? {}) as Record<string, unknown>;

      switch (ev.type) {
        case "oneshot_run_started": {
          const promptIds = Array.isArray(p.promptIds) ? (p.promptIds as string[]) : [];
          return {
            ...state,
            runId: (p.runId as string) ?? state.runId,
            status: "warming_up",
            model: (p.model as string) ?? state.model,
            promptIds,
            prompts: Object.fromEntries(
              promptIds.map((id) => [id, { id, status: "pending", output: "" }])
            ),
            lastSeenSeq: ev.seq,
          };
        }

        case "oneshot_warmup_started":
          return { ...state, status: "warming_up", lastSeenSeq: ev.seq };

        case "oneshot_warmup_finished":
          return { ...state, status: "running", lastSeenSeq: ev.seq };

        case "oneshot_test_started": {
          const promptId = (p.promptId as string) ?? "";
          return {
            ...state,
            prompts: {
              ...state.prompts,
              [promptId]: { ...state.prompts[promptId], status: "running", output: "" },
            },
            lastSeenSeq: ev.seq,
          };
        }

        case "oneshot_delta": {
          const promptId = (p.promptId as string) ?? "";
          const content = (p.content as string) ?? "";
          const existing = state.prompts[promptId];
          if (!existing) return { ...state, lastSeenSeq: ev.seq };
          return {
            ...state,
            prompts: {
              ...state.prompts,
              [promptId]: { ...existing, output: existing.output + content },
            },
            lastSeenSeq: ev.seq,
          };
        }

        case "oneshot_test_finished": {
          const promptId = (p.promptId as string) ?? "";
          const output = (p.output as string) ?? "";
          const existing = state.prompts[promptId];
          const metrics = (p.metrics as Record<string, unknown>) ?? {};
          return {
            ...state,
            prompts: {
              ...state.prompts,
              [promptId]: {
                ...existing,
                id: promptId,
                status: p.error ? "failed" : "done",
                output: output || existing?.output || "",
                finishReason: (p.finishReason as string) ?? undefined,
                wallTimeMs: (p.wallTimeMs as number) ?? undefined,
                firstTokenMs: (p.firstTokenMs as number) ?? undefined,
                promptTokens: (metrics.promptTokens as number) ?? undefined,
                completionTokens: (metrics.completionTokens as number) ?? undefined,
                artifact: (p.artifact as boolean) ?? false,
                artifactVersion: ((p.artifact as boolean) ? ((existing?.artifactVersion ?? 0) + 1) : existing?.artifactVersion),
                error: (p.error as string) ?? undefined,
              },
            },
            lastSeenSeq: ev.seq,
          };
        }

        case "oneshot_run_finished":
          return { ...state, status: "done", lastSeenSeq: ev.seq };

        case "oneshot_run_stopped":
          return { ...state, status: "stopped", lastSeenSeq: ev.seq };

        case "oneshot_run_failed":
          return { ...state, status: "failed", lastSeenSeq: ev.seq };

        default:
          return { ...state, lastSeenSeq: ev.seq };
      }
    }

    case "stop":
      return { ...state, status: "stopped" };

    case "reset":
      return initialState;

    default:
      return state;
  }
}

export const initialState: OneshotState = {
  runId: null,
  status: "idle",
  model: null,
  promptIds: [],
  prompts: {},
  lastSeenSeq: -1,
};

export function hydrateFromLatestRun(
  runId: string,
  status: string,
  model: string | null | undefined,
  promptIds: string[],
  results: OneshotResult[]
): OneshotState {
  const prompts: Record<string, OneshotPromptState> = {};
  
  for (const r of results) {
    prompts[r.promptId] = {
      id: r.promptId,
      status: r.status,
      output: r.output ?? "",
      model: r.model,
      finishReason: r.finishReason,
      wallTimeMs: r.wallTimeMs,
      firstTokenMs: r.firstTokenMs,
      promptTokens: r.promptTokens,
      completionTokens: r.completionTokens,
      artifact: r.hasArtifact,
      artifactVersion: r.hasArtifact ? 1 : 0,
      error: r.error,
    };
  }

  let mappedStatus: OneshotState["status"] = "idle";
  if (status === "warming_up") mappedStatus = "warming_up";
  else if (status === "running") mappedStatus = "running";
  else if (status === "done") mappedStatus = "done";
  else if (status === "failed") mappedStatus = "failed";
  else if (status === "stopped") mappedStatus = "stopped";

  return {
    runId,
    status: mappedStatus,
    model: model ?? null,
    promptIds,
    prompts,
    lastSeenSeq: -1,
  };
}
