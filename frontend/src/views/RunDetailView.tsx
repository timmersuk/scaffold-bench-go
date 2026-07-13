import { useEffect, useState, useReducer } from "react";
import { ArrowLeft } from "lucide-react";
import { api } from "../api";
import { normalizeBackendEvent, type BackendEvent } from "../types";
import { reducer, INITIAL_REDUCER_STATE } from "../hooks/run-state-reducer";
import { ScenarioQueue } from "../components/ScenarioQueue";
import { LogTerminal } from "../components/LogTerminal";
import { MetricsPanel } from "../components/MetricsPanel";
import { VerificationPanel } from "../components/VerificationPanel";
import type { ReducerState } from "../hooks/run-state-reducer";
import {
  getFocusedScenario,
  getCategoryRollups,
  getDisplayedPoints,
  getModel,
  getCallCounts,
  isRunComplete,
} from "./dashboard-selectors";

interface RunDetailViewProps {
  runId: string;
  onBack: () => void;
}

type ViewState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "ready" };

export function RunDetailView({ runId, onBack }: RunDetailViewProps) {
  const [state, dispatch] = useReducer(reducer, INITIAL_REDUCER_STATE);
  const [viewState, setViewState] = useState<ViewState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    const load = async () => {
      try {
        const [detail, events] = await Promise.all([
          api.getRun(runId, controller.signal),
          api.getRunEvents(runId, -1, controller.signal),
        ]);
        if (cancelled) return;

        dispatch({ type: "_reset" });

        for (const raw of events) {
          const event = normalizeBackendEvent(raw as BackendEvent);
          if (event) dispatch(event);
        }

        // If the run was persisted from a version that did not emit run_finished,
        // synthesize final totals from the API detail so the header/footer match.
        const currentStatus = state.status;
        if (currentStatus !== "done" && currentStatus !== "stopped" && currentStatus !== "failed") {
          const derivedStatus: ReducerState["status"] =
            detail.status === "running" ? "done" : (detail.status as ReducerState["status"]);
          dispatch({
            type: "run_finished",
            seq: -1,
            ts: Date.now(),
            runId: detail.id,
            totalPoints: detail.totalPoints,
            maxPoints: detail.maxPoints,
          });
          if (derivedStatus === "stopped") {
            dispatch({ type: "run_stopped", seq: -1, ts: Date.now(), runId: detail.id });
          } else if (derivedStatus === "failed") {
            dispatch({ type: "run_failed", seq: -1, ts: Date.now(), runId: detail.id, error: "" });
          }
        }

        setViewState({ kind: "ready" });
      } catch (err) {
        if (cancelled) return;
        setViewState({
          kind: "error",
          message: err instanceof Error ? err.message : "Failed to load run",
        });
      }
    };

    load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [runId]);

  const focusedScenario = getFocusedScenario(state);
  const focusedId = state.focusedScenarioId ?? state.activeScenarioId;
  const metrics = focusedScenario?.liveMetrics ?? state.globalMetrics;
  const callCounts = getCallCounts(focusedScenario);
  const categoryRollups = getCategoryRollups(state);
  const displayed = getDisplayedPoints(state);
  const model = getModel(state, focusedScenario);
  const runComplete = isRunComplete(state.status);

  return (
    <div className="flex flex-col h-[calc(100vh-5rem)] min-h-0">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
          >
            <ArrowLeft size={16} />
            Back to history
          </button>
          <h2 className="text-lg font-semibold">Run {runId}</h2>
        </div>
        <div className="text-sm text-gray-500">
          Model: <span className="font-medium text-gray-900">{model ?? "—"}</span>
          <span className="mx-2 text-gray-300">|</span>
          Score: {displayed.total} / {displayed.max} pts
        </div>
      </div>

      {viewState.kind === "loading" && (
        <div className="flex-1 flex items-center justify-center text-gray-500 gap-2">
          <span className="w-5 h-5 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
          Loading run replay…
        </div>
      )}

      {viewState.kind === "error" && (
        <div className="flex-1 flex items-center justify-center">
          <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700 max-w-md">
            {viewState.message}
          </div>
        </div>
      )}

      {viewState.kind === "ready" && (
        <div className="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-4">
          <div className="md:col-span-3 min-h-0">
            <ScenarioQueue
              scenarios={state.scenarios}
              focusedId={focusedId}
              onFocus={(id) => dispatch({ type: "_focus", id })}
            />
          </div>

          <div className="md:col-span-6 min-h-0">
            <LogTerminal scenario={focusedScenario} isLive={false} />
          </div>

          <div className="md:col-span-3 flex flex-col gap-4 min-h-0">
            <MetricsPanel
              metrics={metrics}
              toolCount={callCounts.tool}
              bashCalls={callCounts.bash}
              editCalls={callCounts.edit}
              firstTokenMs={focusedScenario?.firstTokenMs}
              turnWallTimes={focusedScenario?.turnWallTimes}
              turnFirstTokenMs={focusedScenario?.turnFirstTokenMs}
            />
            <VerificationPanel
              scenario={focusedScenario}
              isRunComplete={runComplete}
              categoryRollups={categoryRollups}
              totalPoints={displayed.total}
              maxPoints={displayed.max}
            />
          </div>
        </div>
      )}
    </div>
  );
}
