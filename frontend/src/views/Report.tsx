import { useEffect, useState } from "react";
import { ArrowLeft, Trophy, Zap, Clock, TrendingUp } from "lucide-react";
import { api } from "../api";
import type { ReportData, ReportModelAggregate } from "../types";

interface ReportProps {
  onBack: () => void;
}

type ViewState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "empty" }
  | { kind: "ready"; data: ReportData };

export function Report({ onBack }: ReportProps) {
  const [state, setState] = useState<ViewState>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();

    api
      .getReportData(controller.signal)
      .then((data) => {
        if (cancelled) return;
        if (data.models.length === 0) {
          setState({ kind: "empty" });
        } else {
          setState({ kind: "ready", data });
        }
      })
      .catch((err) => {
        if (cancelled) return;
        setState({
          kind: "error",
          message: err instanceof Error ? err.message : "Failed to load report",
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
            <h2 className="text-lg font-semibold">Leaderboard</h2>
          </div>
        </div>

        {state.kind === "loading" && (
          <div className="mt-8 flex items-center justify-center gap-2 text-gray-500">
            <span className="w-4 h-4 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
            Loading report…
          </div>
        )}

        {state.kind === "error" && (
          <div className="mt-8 rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">
            {state.message}
          </div>
        )}

        {state.kind === "empty" && (
          <div className="mt-8 text-center text-gray-500">
            <p>No benchmark data available yet.</p>
            <p className="text-sm mt-1">Complete some runs to see the leaderboard.</p>
          </div>
        )}

        {state.kind === "ready" && (
          <div className="mt-6 space-y-6">
            <Awards data={state.data} />
            <Totals data={state.data} />
            <Leaderboard models={state.data.models} />
          </div>
        )}
      </div>
    </div>
  );
}

function Awards({ data }: { data: ReportData }) {
  const awards = data.awards;
  if (!awards.bestOverall && !awards.bestEfficiency && !awards.fastestGeneration && !awards.fastestPrompt) {
    return null;
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
      {awards.bestOverall && (
        <AwardCard
          icon={<Trophy className="text-yellow-500" size={20} />}
          title="Best Overall"
          model={awards.bestOverall.model}
          value={`${awards.bestOverall.solveRatePct.toFixed(1)}% solve rate`}
        />
      )}
      {awards.bestEfficiency && (
        <AwardCard
          icon={<Zap className="text-green-500" size={20} />}
          title="Best Efficiency"
          model={awards.bestEfficiency.model}
          value={`${(awards.bestEfficiency.scorePct / awards.bestEfficiency.avgScenarioSeconds).toFixed(1)} pts/sec`}
        />
      )}
      {awards.fastestGeneration && (
        <AwardCard
          icon={<Clock className="text-blue-500" size={20} />}
          title="Fastest Generation"
          model={awards.fastestGeneration.model}
          value={`${awards.fastestGeneration.completionTps?.toFixed(1)} tok/s`}
        />
      )}
      {awards.fastestPrompt && (
        <AwardCard
          icon={<TrendingUp className="text-purple-500" size={20} />}
          title="Fastest Prompt"
          model={awards.fastestPrompt.model}
          value={`${awards.fastestPrompt.promptTps?.toFixed(1)} tok/s`}
        />
      )}
    </div>
  );
}

function AwardCard({ icon, title, model, value }: { icon: React.ReactNode; title: string; model: string; value: string }) {
  return (
    <div className="rounded-lg border bg-gray-50 p-4">
      <div className="flex items-center gap-2 mb-2">
        {icon}
        <span className="text-sm font-medium text-gray-700">{title}</span>
      </div>
      <div className="text-sm font-semibold text-gray-900 truncate">{model}</div>
      <div className="text-xs text-gray-500 mt-1">{value}</div>
    </div>
  );
}

function Totals({ data }: { data: ReportData }) {
  const totals = data.totals;
  return (
    <div className="grid grid-cols-2 md:grid-cols-5 gap-4 text-center">
      <div className="rounded-lg border bg-white p-3">
        <div className="text-2xl font-bold text-gray-900">{totals.models}</div>
        <div className="text-xs text-gray-500">Models</div>
      </div>
      <div className="rounded-lg border bg-white p-3">
        <div className="text-2xl font-bold text-gray-900">{totals.runs}</div>
        <div className="text-xs text-gray-500">Runs</div>
      </div>
      <div className="rounded-lg border bg-white p-3">
        <div className="text-2xl font-bold text-gray-900">{totals.scenarioRuns}</div>
        <div className="text-xs text-gray-500">Scenario Runs</div>
      </div>
      <div className="rounded-lg border bg-white p-3">
        <div className="text-2xl font-bold text-gray-900">{totals.local}</div>
        <div className="text-xs text-gray-500">Local</div>
      </div>
      <div className="rounded-lg border bg-white p-3">
        <div className="text-2xl font-bold text-gray-900">{totals.remote}</div>
        <div className="text-xs text-gray-500">Remote</div>
      </div>
    </div>
  );
}

function Leaderboard({ models }: { models: ReportModelAggregate[] }) {
  return (
    <div className="overflow-hidden rounded-lg border border-gray-200">
      <table className="w-full text-sm text-left">
        <thead className="bg-gray-50 text-xs uppercase text-gray-500">
          <tr>
            <th className="px-4 py-3 font-semibold">Model</th>
            <th className="px-4 py-3 font-semibold text-right">Solve %</th>
            <th className="px-4 py-3 font-semibold text-right">Discipline %</th>
            <th className="px-4 py-3 font-semibold text-right">Score %</th>
            <th className="px-4 py-3 font-semibold text-right">Runs</th>
            <th className="px-4 py-3 font-semibold text-right">Avg Time</th>
            <th className="px-4 py-3 font-semibold text-right">Tokens/Scenario</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {models.map((model, idx) => (
            <tr key={idx} className="hover:bg-gray-50">
              <td className="px-4 py-3">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-gray-900">{model.model}</span>
                  <span className={`text-xs px-2 py-0.5 rounded ${model.source === "local" ? "bg-blue-100 text-blue-700" : "bg-purple-100 text-purple-700"}`}>
                    {model.source}
                  </span>
                  {model.paretoFrontier && (
                    <span className="text-xs px-2 py-0.5 rounded bg-green-100 text-green-700">
                      Pareto
                    </span>
                  )}
                </div>
              </td>
              <td className="px-4 py-3 text-right">
                <div className="font-semibold text-gray-900">{model.solveRatePct.toFixed(1)}%</div>
                <div className="text-xs text-gray-500">
                  [{model.solveCiLowPct.toFixed(1)}–{model.solveCiHighPct.toFixed(1)}]
                </div>
              </td>
              <td className="px-4 py-3 text-right text-gray-700">{model.disciplinePct.toFixed(1)}%</td>
              <td className="px-4 py-3 text-right text-gray-700">{model.scorePct.toFixed(1)}%</td>
              <td className="px-4 py-3 text-right text-gray-700">{model.runs}</td>
              <td className="px-4 py-3 text-right text-gray-700">{model.avgScenarioSeconds.toFixed(1)}s</td>
              <td className="px-4 py-3 text-right text-gray-700">{model.avgTokensPerScenario.toFixed(0)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
