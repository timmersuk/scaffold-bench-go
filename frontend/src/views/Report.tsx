import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { api } from "../api";
import { formatTokenCount, formatTps, formatSeconds, formatWallTime } from "../lib/format";
import { scoreTextColor } from "../lib/score-color";
import type { ReportData, ReportModelAggregate } from "../types";
import {
  CategoryHeatmap,
  ContextGrowthChart,
  MetricBars,
  TokenScoreScatter,
  sortByScore,
  sortByMetric,
} from "../components/report";

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
            Loading report&hellip;
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
            <CategoryHeatmap
              models={sortByScore(state.data.models)}
              columns={state.data.categories}
              scoreFor={(model) => model.categories}
              title="Category heatmap (%)"
            />
            <CategoryHeatmap
              models={sortByScore(state.data.models)}
              columns={["low", "medium", "high"]}
              scoreFor={(model) => model.tiers}
              title="Score by difficulty (%)"
            />
            <MetricBars
              title="Quality score (% of scored max)"
              models={sortByScore(state.data.models)}
              value={(model) => model.scorePct}
              format={(value) => `${value.toFixed(1)}%`}
              color="#2ECC71"
            />
            <MetricBars
              title="Generation speed (completion tok/s)"
              models={sortByMetric(state.data.models, (model) => model.completionTps)}
              value={(model) => model.completionTps}
              format={(value, model) =>
                `${model.completionTpsApprox ? "~" : ""}${value.toFixed(1)}`
              }
              color="#3498DB"
            />
            <MetricBars
              title="Prompt processing speed (prompt eval tok/s)"
              models={sortByMetric(state.data.models, (model) => model.promptTps)}
              value={(model) => model.promptTps}
              format={(value, model) => `${model.promptTpsApprox ? "~" : ""}${value.toFixed(0)}`}
              color="#FFBF00"
            />
            <MetricBars
              title="Scenario avg time (s)"
              models={sortByMetric(state.data.models, (model) => model.avgScenarioSeconds, true)}
              value={(model) => model.avgScenarioSeconds}
              format={(value) => `${value.toFixed(1)}s`}
              color="#E74C3C"
              lowerIsBetter
            />
            <MetricBars
              title="TTFT \u00b7 time to first token (s)"
              models={sortByMetric(state.data.models, (model) => model.avgFirstTokenSeconds, true)}
              value={(model) => model.avgFirstTokenSeconds}
              format={(value) => `${value.toFixed(2)}s`}
              color="#b38bff"
              lowerIsBetter
            />
            <MetricBars
              title="Tokens per scenario (lower = cheaper)"
              models={sortByMetric(state.data.models, (model) => model.avgTokensPerScenario, true)}
              value={(model) => model.avgTokensPerScenario}
              format={(value) => (value > 0 ? formatTokenCount(value) : "\u2014")}
              color="#9b59b6"
              lowerIsBetter
            />
            <MetricBars
              title="Context per turn (lower = tighter)"
              models={sortByMetric(state.data.models, (model) => model.avgContextPerTurn, true)}
              value={(model) => model.avgContextPerTurn}
              format={(value, model) => {
                if (value === null) return "\u2014";
                const base = formatTokenCount(value);
                if (!model.contextPerTurnByHarness) return base;
                const split = Object.entries(model.contextPerTurnByHarness)
                  .map(([h, v]) => `${h} ${formatTokenCount(v)}`)
                  .join(" \u00b7 ");
                return `${base}  (${split})`;
              }}
              color="#e8590c"
              lowerIsBetter
            />
            <div className="flex flex-wrap gap-x-8 items-start">
              <div className="flex-1 min-w-[480px]">
                <ContextGrowthChart models={sortByScore(state.data.models)} />
              </div>
              <div className="flex-1 min-w-[480px]">
                <TokenScoreScatter models={sortByScore(state.data.models)} cloud={state.data.pareto} />
              </div>
            </div>
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
          title="Best Overall"
          model={awards.bestOverall.model}
          value={`${awards.bestOverall.scorePct.toFixed(1)}% \u00b7 ${formatTps(awards.bestOverall.completionTps, awards.bestOverall.completionTpsApprox, 1)} gen tps`}
        />
      )}
      {awards.bestEfficiency && (
        <AwardCard
          title="Best Efficiency"
          model={awards.bestEfficiency.model}
          value={`${awards.bestEfficiency.scorePct.toFixed(1)}% @ ${awards.bestEfficiency.avgScenarioSeconds.toFixed(1)}s/scen`}
        />
      )}
      {awards.fastestGeneration && (
        <AwardCard
          title="Fastest Generation"
          model={awards.fastestGeneration.model}
          value={`${formatTps(awards.fastestGeneration.completionTps, awards.fastestGeneration.completionTpsApprox, 1)} gen tps`}
        />
      )}
      {awards.fastestPrompt && (
        <AwardCard
          title="Fastest Prompt"
          model={awards.fastestPrompt.model}
          value={`${formatTps(awards.fastestPrompt.promptTps, awards.fastestPrompt.promptTpsApprox, 0)} prompt tps`}
        />
      )}
    </div>
  );
}

function AwardCard({ title, model, value }: { title: string; model: string; value: string }) {
  return (
    <div className="rounded-lg border bg-gray-50 p-4">
      <div className="text-xs text-gray-500 uppercase tracking-widest">{title}</div>
      <div className="text-sm font-semibold text-gray-900 truncate mt-1">{model}</div>
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

type SortKey =
  | "solve"
  | "verify"
  | "ptsPerRun"
  | "genTps"
  | "promptTps"
  | "scenAvg"
  | "tokens"
  | "totalWall"
  | "ttft"
  | "tools"
  | "requests"
  | "timeouts"
  | "runs";

type SortDir = "asc" | "desc";

const VERIFY_TOOLTIP =
  "% of runs where the model ran a passing test/typecheck after changing code. Behavioral \u2014 measured from the tool-call trace, independent of rubric points. Distinct from the graded verification dimension inside Discipline %.";

const COLUMNS: Record<SortKey, { label: string; align: string; title?: string }> = {
  solve: { label: "Score", align: "text-right" },
  verify: { label: "Verify %", align: "text-right", title: VERIFY_TOOLTIP },
  ptsPerRun: { label: "Pts/run", align: "text-right" },
  genTps: { label: "Gen TPS", align: "text-right" },
  promptTps: { label: "Prompt TPS", align: "text-right" },
  scenAvg: { label: "Scen Avg", align: "text-right" },
  tokens: { label: "Tokens/scen", align: "text-right" },
  totalWall: { label: "Total Wall", align: "text-right" },
  ttft: { label: "TTFT", align: "text-right" },
  tools: { label: "Tools", align: "text-right" },
  requests: { label: "Requests", align: "text-right" },
  timeouts: { label: "T/O", align: "text-right" },
  runs: { label: "Runs", align: "text-right" },
};

function compareModels(key: SortKey, a: ReportModelAggregate, b: ReportModelAggregate): number {
  const nullSort = (v: number | null): [boolean, number] =>
    v === null || v === undefined ? [true, 0] : [false, v];

  switch (key) {
    case "solve":
      return a.solveRatePct - b.solveRatePct;
    case "verify": {
      const [aNull, aVal] = nullSort(a.verifyRatePct);
      const [bNull, bVal] = nullSort(b.verifyRatePct);
      if (aNull && bNull) return 0;
      if (aNull) return 1;
      if (bNull) return -1;
      return aVal - bVal;
    }
    case "ptsPerRun":
      return a.pointsAvg - b.pointsAvg;
    case "genTps": {
      const [aNull, aVal] = nullSort(a.completionTps);
      const [bNull, bVal] = nullSort(b.completionTps);
      if (aNull && bNull) return 0;
      if (aNull) return 1;
      if (bNull) return -1;
      return aVal - bVal;
    }
    case "promptTps": {
      const [aNull, aVal] = nullSort(a.promptTps);
      const [bNull, bVal] = nullSort(b.promptTps);
      if (aNull && bNull) return 0;
      if (aNull) return 1;
      if (bNull) return -1;
      return aVal - bVal;
    }
    case "scenAvg":
      return a.avgScenarioSeconds - b.avgScenarioSeconds;
    case "tokens":
      return a.avgTokensPerScenario - b.avgTokensPerScenario;
    case "totalWall":
      return a.totalWallSeconds - b.totalWallSeconds;
    case "ttft": {
      const [aNull, aVal] = nullSort(a.avgFirstTokenSeconds);
      const [bNull, bVal] = nullSort(b.avgFirstTokenSeconds);
      if (aNull && bNull) return 0;
      if (aNull) return 1;
      if (bNull) return -1;
      return aVal - bVal;
    }
    case "tools":
      return a.toolCallsTotal - b.toolCallsTotal;
    case "requests":
      return a.requests - b.requests;
    case "timeouts":
      return a.timeouts - b.timeouts;
    case "runs":
      return a.runs - b.runs;
  }
}

function Leaderboard({ models }: { models: ReportModelAggregate[] }) {
  const [sortKey, setSortKey] = useState<SortKey>("solve");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const handleSort = (key: SortKey) => {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  };

  const sorted = [...models].sort((a, b) => {
    const cmp = compareModels(sortKey, a, b);
    return sortDir === "asc" ? cmp : -cmp;
  });

  const arrow = (key: SortKey) => {
    if (key !== sortKey) return null;
    return <span className="ml-1">{sortDir === "asc" ? "\u25B2" : "\u25BC"}</span>;
  };

  const sortableTh = (key: SortKey) => {
    const col = COLUMNS[key];
    return (
      <th
        key={key}
        className={`${col.align} px-2 py-2 cursor-pointer select-none hover:text-gray-900 transition-colors`}
        onClick={() => handleSort(key)}
        title={col.title}
      >
        {col.label}
        {arrow(key)}
      </th>
    );
  };

  return (
    <div className="overflow-x-auto border border-gray-200 rounded-lg">
      <table className="w-full text-xs border-collapse">
        <thead>
          <tr className="border-b border-gray-200 text-[10px] uppercase tracking-widest text-gray-500 bg-gray-100">
            <th className="text-left px-2 py-2">#</th>
            <th className="text-left px-2 py-2">Model</th>
            <th className="text-left px-2 py-2">Src</th>
            {sortableTh("solve")}
            {sortableTh("verify")}
            {sortableTh("ptsPerRun")}
            {sortableTh("genTps")}
            {sortableTh("promptTps")}
            {sortableTh("scenAvg")}
            {sortableTh("tokens")}
            {sortableTh("totalWall")}
            {sortableTh("ttft")}
            {sortableTh("tools")}
            {sortableTh("requests")}
            {sortableTh("timeouts")}
            {sortableTh("runs")}
          </tr>
        </thead>
        <tbody>
          {sorted.map((model, index) => (
            <tr key={model.model} className="border-b border-gray-100 hover:bg-gray-50">
              <td className="px-2 py-2 text-gray-900">{index + 1}</td>
              <td className="px-2 py-2 text-gray-900 font-bold max-w-[260px] truncate">
                {model.model}
              </td>
              <td className="px-2 py-2">
                <SourceBadge source={model.source} />
              </td>
              <td className={`px-2 py-2 text-right font-bold ${scoreTextColor(model.solveRatePct)}`}>
                {model.solveRatePct.toFixed(1)}%
                <span className="ml-1 font-normal text-[10px] text-gray-400">
                  &plusmn;{((model.solveCiHighPct - model.solveCiLowPct) / 2).toFixed(1)}
                </span>
              </td>
              <td className="px-2 py-2 text-right text-gray-900" title={VERIFY_TOOLTIP}>
                {model.verifyEligibleRuns > 0 && model.verifyRatePct !== null
                  ? `${model.verifyRatePct.toFixed(0)}%`
                  : "\u2014"}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {model.pointsAvg.toFixed(1)} / {model.maxAvg.toFixed(0)}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {formatTps(model.completionTps, model.completionTpsApprox, 1)}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {formatTps(model.promptTps, model.promptTpsApprox, 0)}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {model.avgScenarioSeconds.toFixed(1)}s
              </td>
              <td className="px-2 py-2 text-right text-gray-900 tabular-nums">
                {model.avgTokensPerScenario > 0
                  ? formatTokenCount(model.avgTokensPerScenario)
                  : "\u2014"}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {formatWallTime(model.totalWallSeconds)}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">
                {formatSeconds(model.avgFirstTokenSeconds, 2)}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">{model.toolCallsTotal}</td>
              <td className="px-2 py-2 text-right text-gray-900">{model.requests}</td>
              <td
                className={`px-2 py-2 text-right ${model.timeouts > 0 ? "text-red-600 font-bold" : "text-gray-400"}`}
              >
                {model.timeouts}
              </td>
              <td className="px-2 py-2 text-right text-gray-900">{model.runs}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SourceBadge({ source }: { source: string }) {
  const color =
    source === "remote"
      ? "text-yellow-700 border-yellow-300 bg-yellow-50"
      : "text-green-700 border-green-300 bg-green-50";
  return (
    <span
      className={`inline-block rounded-sm border px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-widest ${color}`}
    >
      {source}
    </span>
  );
}
