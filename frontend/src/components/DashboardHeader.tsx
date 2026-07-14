import type { RunStatus } from "../types";
import { formatElapsed } from "../lib/format";
import { Play, Square, History } from "lucide-react";

interface DashboardHeaderProps {
  totalPoints: number;
  maxPoints: number;
  elapsed: number;
  status: RunStatus;
  onStart: () => void;
  onStop: () => void;
  onHistory: () => void;
}

const BADGE_STYLES: Record<RunStatus, string> = {
  idle: "border-gray-300 text-gray-500",
  warming_up: "border-yellow-600 text-yellow-600 animate-pulse",
  running: "border-blue-600 text-blue-600 animate-pulse",
  done: "border-green-600 text-green-600",
  stopped: "border-red-600 text-red-600",
  failed: "border-red-600 text-red-600",
};

export function DashboardHeader({
  totalPoints,
  maxPoints,
  elapsed,
  status,
  onStart,
  onStop,
  onHistory,
}: DashboardHeaderProps) {
  const isRunning = status === "running" || status === "warming_up";

  return (
    <header className="flex flex-col md:flex-row gap-4 justify-between items-start md:items-center mb-4 pb-4 border-b border-gray-200 flex-none">
      <div className="flex flex-col">
        <h2 className="text-lg font-bold tracking-tight text-gray-900 leading-none">Dashboard</h2>
        <p className="text-[10px] text-gray-500 uppercase tracking-widest mt-0.5">
          Run benchmark and watch live progress
        </p>
      </div>

      <div className="flex flex-wrap gap-5 items-center">
        <div className="flex flex-col items-end">
          <span className="text-[10px] text-gray-500 uppercase tracking-widest">Score</span>
          <span className="text-[17px] font-bold text-green-700 leading-tight">
            {totalPoints} <span className="text-gray-500 text-sm font-normal">/ {maxPoints} pts</span>
          </span>
        </div>

        <div className="w-px h-8 bg-gray-200" />

        <div className="flex flex-col items-end">
          <span className="text-[10px] text-gray-500 uppercase tracking-widest">Elapsed</span>
          <span className="text-[17px] font-bold text-gray-900 leading-tight">{formatElapsed(elapsed)}</span>
        </div>

        <div className="w-px h-8 bg-gray-200" />

        <span className={`px-2 py-0.5 text-[10px] uppercase border rounded-sm ${BADGE_STYLES[status]}`}>
          {status.toUpperCase()}
        </span>

        <div className="w-px h-8 bg-gray-200 hidden md:block" />

        <div className="flex gap-2">
          <button
            onClick={onStart}
            disabled={isRunning}
            className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] uppercase tracking-wider border border-gray-200 bg-white text-gray-600 hover:border-blue-600 hover:text-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors rounded-md"
          >
            <Play size={12} />
            Start
          </button>
          <button
            onClick={onStop}
            disabled={!isRunning}
            className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] uppercase tracking-wider border border-gray-200 bg-white text-gray-600 hover:border-red-600 hover:text-red-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors rounded-md"
          >
            <Square size={12} />
            Stop
          </button>
          <button
            onClick={onHistory}
            className="flex items-center gap-1.5 px-3 py-1.5 text-[11px] uppercase tracking-wider border border-gray-200 bg-white text-gray-600 hover:border-gray-400 hover:text-gray-800 transition-colors rounded-md"
          >
            <History size={12} />
            History
          </button>
        </div>
      </div>
    </header>
  );
}
