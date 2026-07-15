import { Play } from "lucide-react";
import type { OneshotTestSummary } from "../types";
import type { OneshotPromptState } from "../hooks/oneshot-state-reducer";

type Props = {
  prompts: OneshotTestSummary[];
  promptStates: Record<string, OneshotPromptState>;
  selectedPromptId: string | null;
  onSelect: (promptId: string) => void;
  onRunSingle: (promptId: string) => void;
  isRunning: boolean;
};

export function OneshotQueue({
  prompts,
  promptStates,
  selectedPromptId,
  onSelect,
  onRunSingle,
  isRunning,
}: Props) {
  return (
    <div className="space-y-1">
      {prompts.map((p) => {
        const state = promptStates[p.id];
        const status = state?.status ?? "pending";
        const isSelected = selectedPromptId === p.id;

        return (
          <div
            key={p.id}
            onClick={() => onSelect(p.id)}
            className={`flex cursor-pointer items-center justify-between rounded-md px-3 py-2 text-sm transition-colors ${
              isSelected ? "bg-blue-50 border border-blue-200" : "hover:bg-gray-50 border border-transparent"
            }`}
          >
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <StatusPill status={status} />
              <span className="truncate font-mono text-xs">{p.id}</span>
            </div>
            <button
              onClick={(e) => {
                e.stopPropagation();
                onRunSingle(p.id);
              }}
              disabled={isRunning}
              className="ml-2 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 disabled:opacity-50"
              title="Run this prompt"
            >
              <Play size={14} />
            </button>
          </div>
        );
      })}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const colors: Record<string, string> = {
    pending: "bg-gray-200 text-gray-600",
    running: "bg-blue-100 text-blue-700 animate-pulse",
    done: "bg-green-100 text-green-700",
    failed: "bg-red-100 text-red-700",
    stopped: "bg-yellow-100 text-yellow-700",
  };

  return (
    <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${colors[status] ?? colors.pending}`}>
      {status}
    </span>
  );
}
