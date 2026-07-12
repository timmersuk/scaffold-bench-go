import { useEffect, useRef, useState, useReducer, useCallback } from "react";
import { api } from "../api";
import { normalizeBackendEvent, type BackendEvent, type PersistedEvent } from "../types";
import { reducer, INITIAL_REDUCER_STATE } from "../hooks/run-state-reducer";
import { useToast } from "../components/Toaster";
import { DashboardHeader } from "../components/DashboardHeader";
import { ScenarioQueue } from "../components/ScenarioQueue";
import { LogTerminal } from "../components/LogTerminal";
import { MetricsPanel } from "../components/MetricsPanel";
import { VerificationPanel } from "../components/VerificationPanel";
import {
  getFocusedScenario,
  getCategoryRollups,
  getDisplayedPoints,
  getModel,
  getCallCounts,
  isRunComplete,
} from "./dashboard-selectors";

const POLL_MS = 5000;

interface DashboardProps {
  onStartRun: () => void;
  onHistory: () => void;
}

export function Dashboard({ onStartRun, onHistory }: DashboardProps) {
  const [state, dispatch] = useReducer(reducer, INITIAL_REDUCER_STATE);
  const [activeRunId, setActiveRunId] = useState<string | null>(null);
  const [connectionState, setConnectionState] = useState<"idle" | "connecting" | "open" | "error">("idle");
  const [healthStatus, setHealthStatus] = useState<"ok" | "error">("ok");
  const lastSeqRef = useRef<number>(-1);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const { pushToast } = useToast();

  const dispatchEvent = useCallback((event: PersistedEvent) => {
    dispatch(event);
    if (event.seq > lastSeqRef.current) {
      lastSeqRef.current = event.seq;
    }
  }, []);


  // Health / active-run polling.
  useEffect(() => {
    let cancelled = false;
    let lastKnownRunId: string | null = null;

    const tick = async () => {
      try {
        const health = await fetch("/api/health");
        if (cancelled) return;
        setHealthStatus(health.ok ? "ok" : "error");
      } catch {
        if (cancelled) return;
        setHealthStatus("error");
      }
      try {
        const { runId } = await api.activeRun();
        if (cancelled) return;
        if (runId !== lastKnownRunId) {
          lastKnownRunId = runId;
          setActiveRunId(runId);
        }
      } catch (err) {
        if (cancelled) return;
        pushToast("Failed to poll active run", "error");
      }
    };

    tick();
    const interval = window.setInterval(tick, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [pushToast]);

  // Event stream management: catch up via /events, then open SSE; reconnect on drops.
  useEffect(() => {
    let cancelled = false;
    let es: EventSource | null = null;
    let reconnectDelay = 1000;

    const clearReconnect = () => {
      if (reconnectTimeoutRef.current) {
        window.clearTimeout(reconnectTimeoutRef.current);
        reconnectTimeoutRef.current = null;
      }
    };

    const applyEvents = async (runId: string, fromSeq: number) => {
      const events = await api.getRunEvents(runId, fromSeq);
      for (const raw of events) {
        const event = normalizeBackendEvent(raw as BackendEvent);
        if (event) dispatchEvent(event);
      }
    };

    const open = async () => {
      if (!activeRunId || cancelled) return;
      setConnectionState("connecting");
      try {
        await applyEvents(activeRunId, lastSeqRef.current);
      } catch (err) {
        if (cancelled) return;
        pushToast("Failed to fetch missed events", "error");
        setConnectionState("error");
      }
      if (cancelled) return;

      es = new EventSource(`/api/runs/${activeRunId}/stream`);
      es.onopen = () => {
        setConnectionState("open");
        reconnectDelay = 1000;
      };
      es.onmessage = (e) => {
        try {
          const raw = JSON.parse(e.data) as BackendEvent;
          const event = normalizeBackendEvent(raw);
          if (event) dispatchEvent(event);
        } catch {
          // ignore malformed message
        }
      };
      es.onerror = () => {
        setConnectionState("error");
        es?.close();
        es = null;
        if (cancelled) return;
        reconnectTimeoutRef.current = window.setTimeout(() => {
          reconnectDelay = Math.min(reconnectDelay * 2, 30000);
          open();
        }, reconnectDelay);
      };
    };

    if (activeRunId) {
      dispatch({ type: "_reset" });
      lastSeqRef.current = -1;
      open();
    } else {
      setConnectionState("idle");
      dispatch({ type: "_reset" });
      lastSeqRef.current = -1;
    }

    return () => {
      cancelled = true;
      clearReconnect();
      es?.close();
    };
  }, [activeRunId, dispatchEvent, pushToast]);

  const handleStart = () => {
    onStartRun();
  };

  const handleStop = () => {
    if (!activeRunId || state.status !== "running") return;
    api.stopRun(activeRunId).catch((err) => {
      pushToast(err instanceof Error ? err.message : "Failed to stop run", "error");
    });
  };

  const elapsed =
    state.status === "running" && state.startedAt
      ? Date.now() - state.startedAt
      : 0;

  const focusedScenario = getFocusedScenario(state);
  const focusedId = state.focusedScenarioId ?? state.activeScenarioId;
  const isLive = state.status === "running" && focusedId === state.activeScenarioId;
  const metrics = focusedScenario?.liveMetrics ?? state.globalMetrics;
  const callCounts = getCallCounts(focusedScenario);
  const categoryRollups = getCategoryRollups(state);
  const displayed = getDisplayedPoints(state);
  const model = getModel(state, focusedScenario);
  const runComplete = isRunComplete(state.status);

  return (
    <div className="flex flex-col h-[calc(100vh-5rem)] min-h-0">
      <DashboardHeader
        totalPoints={displayed.total}
        maxPoints={displayed.max}
        elapsed={elapsed}
        status={state.status}
        onStart={handleStart}
        onStop={handleStop}
        onHistory={onHistory}
      />

      {activeRunId === null && state.status === "idle" ? (
        <div className="flex-1 flex flex-col items-center justify-center gap-4">
          <div className="text-center">
            <h3 className="text-xl font-semibold text-gray-900">No active benchmark run</h3>
            <p className="mt-2 text-gray-600 max-w-md">
              Start a new run to see the live scenario queue, agent logs, and metrics.
            </p>
          </div>
          <button
            onClick={handleStart}
            className="flex items-center gap-2 px-6 py-3 bg-blue-600 text-white rounded-md font-medium hover:bg-blue-700 transition-colors shadow-sm"
          >
            Start Run
          </button>
        </div>
      ) : (
        <>
          <div className="flex-1 min-h-0 grid grid-cols-1 md:grid-cols-12 gap-4">
            <div className="md:col-span-3 min-h-0">
              <ScenarioQueue
                scenarios={state.scenarios}
                focusedId={focusedId}
                onFocus={(id) => dispatch({ type: "_focus", id })}
              />
            </div>

            <div className="md:col-span-6 min-h-0">
              <LogTerminal scenario={focusedScenario} isLive={isLive} />
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

          <footer className="flex-none flex items-center justify-between h-10 border-t border-gray-200 bg-white px-4 text-[11px] text-gray-500 mt-2">
            <div className="flex items-center gap-3">
              <span className="flex items-center gap-1.5">
                <span
                  className={`w-1.5 h-1.5 rounded-full ${
                    healthStatus === "ok" ? "bg-green-600" : "bg-red-600"
                  }`}
                />
                API {healthStatus === "ok" ? "connected" : "unreachable"}
              </span>
              <span>Model: {model ?? "—"}</span>
              <span>
                Stream: {connectionState}{" "}
                {connectionState === "connecting" && (
                  <span className="inline-block w-2 h-2 border-2 border-blue-600 border-t-transparent rounded-full animate-spin ml-1" />
                )}
              </span>
            </div>
            <div className="flex items-center gap-3">
              <span>Scenarios: {state.scenarios.length}</span>
              <span>Status: {state.status}</span>
            </div>
          </footer>
        </>
      )}
    </div>
  );
}