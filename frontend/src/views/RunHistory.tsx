import { useEffect, useState } from "react";
import { ArrowLeft, Calendar, Cpu, Trophy } from "lucide-react";
import { api } from "../api";
import type { RunSummary } from "../types";

interface RunHistoryProps {
  onBack: () => void;
  onOpenRun: (runId: string) => void;
}

type ViewState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { kind: "ready"; runs: RunSummary[] };

export function RunHistory({ onBack, onOpenRun }: RunHistoryProps) {
  const [state, setState] = useState<ViewState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    api
      .listRuns(controller.signal)
      .then((runs) => {
        if (cancelled) return;
        if (runs.length === 0) {
          setState({ kind: "empty" });
        } else {
          setState({ kind: "ready", runs });
        }
      })
      .catch((err) => {
        if (cancelled) return;
        setState({
          kind: "error",
          message: err instanceof Error ? err.message : "Failed to load run history",
        });
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, []);

  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-white p-6 shadow-sm">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={onBack}
              className="flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
            >
              <ArrowLeft size={16} />
              Back to Dashboard
            </button>
            <h2 className="text-lg font-semibold">Run History</h2>
          </div>
        </div>

        {state.kind === "loading" && (
          <div className="mt-8 flex items-center justify-center gap-2 text-gray-500">
            <span className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
            Loading runs…
          </div>
        )}

        {state.kind === "error" && (
          <div className="mt-8 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {state.message}
          </div>
        )}

        {state.kind === "empty" && (
          <div className="mt-8 text-center text-gray-500">
            <p>No completed benchmark runs yet.</p>
            <p className="text-sm mt-1">Start a run from the dashboard to see it here.</p>
          </div>
        )}

        {state.kind === "ready" && (
          <div className="mt-4 overflow-hidden rounded-lg border border-gray-200">
            <table className="w-full text-sm text-left">
              <thead className="bg-gray-50 text-xs uppercase text-gray-500">
                <tr>
                  <th className="px-4 py-3 font-semibold">Model</th>
                  <th className="px-4 py-3 font-semibold">Scenarios</th>
                  <th className="px-4 py-3 font-semibold">Started</th>
                  <th className="px-4 py-3 font-semibold">Status</th>
                  <th className="px-4 py-3 font-semibold">Score</th>
                  <th className="px-4 py-3 font-semibold" />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {state.runs.map((run) => (
                  <tr key={run.id} className="hover:bg-gray-50">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Cpu size={14} className="text-gray-400" />
                        <span className="font-medium text-gray-900">{run.model}</span>
                      </div>
                      <div className="text-xs text-gray-400 mt-0.5 font-mono">{run.id}</div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm text-gray-700">
                        {run.scenarioIds.length} scenario{run.scenarioIds.length !== 1 ? "s" : ""}
                      </div>
                      <div className="text-xs text-gray-500 mt-0.5 truncate max-w-xs">
                        {run.scenarioIds.slice(0, 3).join(", ")}
                        {run.scenarioIds.length > 3 && ` +${run.scenarioIds.length - 3} more`}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2 text-gray-600">
                        <Calendar size={14} className="text-gray-400" />
                        {formatDate(run.startedAt)}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={run.status} />
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Trophy size={14} className="text-gray-400" />
                        <span className="font-semibold text-gray-900">
                          {run.totalPoints}
                          <span className="text-gray-400 font-normal">/{run.maxPoints}</span>
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-right">
                      <button
                        onClick={() => onOpenRun(run.id)}
                        className="text-xs font-medium text-blue-600 hover:text-blue-700 hover:underline"
                      >
                        Open run
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const base = "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium";
  switch (status) {
    case "done":
      return <span className={`${base} bg-green-100 text-green-700`}>Done</span>;
    case "running":
      return <span className={`${base} bg-blue-100 text-blue-700`}>Running</span>;
    case "failed":
      return <span className={`${base} bg-red-100 text-red-700`}>Failed</span>;
    case "stopped":
      return <span className={`${base} bg-gray-100 text-gray-700`}>Stopped</span>;
    default:
      return <span className={`${base} bg-gray-100 text-gray-700 capitalize`}>{status}</span>;
  }
}

function formatDate(ts: number): string {
  return new Date(ts).toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
