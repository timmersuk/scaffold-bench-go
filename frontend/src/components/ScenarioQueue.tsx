import type { ScenarioState, ScenarioStatus } from "../types";
import { formatElapsed } from "../lib/format";
import { Panel } from "./Panel";

interface ScenarioQueueProps {
  scenarios: ScenarioState[];
  focusedId: string | null;
  onFocus: (id: string) => void;
}

function StatusIcon({ status }: { status: ScenarioStatus }) {
  if (status === "running") return <span className="text-blue-600 animate-pulse">▶</span>;
  if (status === "pending") return <span className="text-gray-400">·</span>;
  if (status === "pass") return <span className="text-green-600">✓</span>;
  if (status === "partial") return <span className="text-yellow-600">◐</span>;
  if (status === "fail") return <span className="text-red-600">✗</span>;
  if (status === "stopped") return <span className="text-gray-400">✗</span>;
  return <span className="text-gray-400">·</span>;
}

function elapsedMs(s: ScenarioState): number {
  if (!s.startedAt) return 0;
  return (s.finishedAt ?? Date.now()) - s.startedAt;
}

export function ScenarioQueue({ scenarios, focusedId, onFocus }: ScenarioQueueProps) {
  const completed = scenarios.filter((s) => s.status !== "pending" && s.status !== "running").length;

  return (
    <Panel title="Queue" rightTag={`${completed} / ${scenarios.length}`} className="h-full">
      <div className="flex-1 overflow-y-auto p-1">
        {scenarios.length === 0 ? (
          <div className="p-4 text-gray-500 text-xs text-center">No scenarios</div>
        ) : (
          scenarios.map((s) => {
            const isRunning = s.status === "running";
            const isFocused = s.id === focusedId;
            const elapsed = elapsedMs(s);

            return (
              <button
                key={s.id}
                onClick={() => onFocus(s.id)}
                className={[
                  "w-full text-left flex flex-col px-3 py-2 border-b border-gray-100 relative",
                  "border-l-2 transition-colors rounded-sm",
                  isRunning
                    ? "bg-blue-50 border-l-blue-600"
                    : isFocused
                      ? "border-l-gray-700 bg-gray-50"
                      : "border-l-transparent hover:bg-gray-50",
                ].join(" ")}
              >
                <div className="flex items-center gap-2 text-xs leading-tight">
                  <span className="w-3 text-center flex-shrink-0 text-sm">
                    <StatusIcon status={s.status} />
                  </span>
                  <span
                    className={`font-bold flex-shrink-0 ${isRunning ? "text-blue-700" : "text-gray-500"}`}
                  >
                    {s.id}
                  </span>
                  <span className="text-gray-900 truncate">{s.name}</span>
                  {isRunning && (
                    <span className="ml-auto flex-shrink-0 w-1.5 h-1.5 rounded-full bg-blue-600 animate-pulse" />
                  )}
                </div>
                <div className="flex items-center gap-2 mt-1 text-[11px] text-gray-500 pl-5">
                  {s.category && (
                    <span className="border border-gray-200 rounded-sm px-1 text-[10px] uppercase text-gray-500">
                      {s.category}
                    </span>
                  )}
                  <span>
                    {s.points !== undefined ? `${s.points}` : "0"}
                    <span className="text-gray-400">/{s.maxPoints}pt</span>
                  </span>
                  {elapsed > 0 ? <span>{formatElapsed(elapsed)}</span> : <span>--:--</span>}
                </div>
              </button>
            );
          })
        )}
      </div>
    </Panel>
  );
}
