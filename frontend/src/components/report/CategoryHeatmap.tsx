import type { ReportModelAggregate, ReportCategoryScore } from "../../types";
import { SectionTitle } from "./SectionTitle";

export function CategoryHeatmap({
  models,
  columns,
  scoreFor,
  title,
}: {
  models: ReportModelAggregate[];
  columns: string[];
  scoreFor: (model: ReportModelAggregate) => Record<string, ReportCategoryScore> | undefined;
  title: string;
}) {
  return (
    <section className="mt-8">
      <SectionTitle>{title}</SectionTitle>
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-xs">
          <thead>
            <tr className="bg-gray-100 text-gray-500 uppercase tracking-widest">
              <th className="text-left border border-gray-200 py-2 px-2">Model</th>
              <th className="text-left border border-gray-200 py-2 px-2">Src</th>
              {columns.map((column) => (
                <th key={column} className="border border-gray-200 py-2 px-2">
                  {column}
                </th>
              ))}
              <th className="border border-gray-200 py-2 px-2">Overall</th>
            </tr>
          </thead>
          <tbody>
            {models.map((model) => {
              const scores = scoreFor(model) ?? {};
              return (
                <tr key={model.model}>
                  <td className="border border-gray-200 py-1.5 px-2 text-gray-900 font-bold whitespace-nowrap">
                    {model.model}
                  </td>
                  <td className="border border-gray-200 py-1.5 px-2">
                    <SourceBadge source={model.source} />
                  </td>
                  {columns.map((column) => (
                    <HeatCell key={column} pct={scores[column]?.pct ?? null} />
                  ))}
                  <HeatCell pct={model.scorePct} />
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function HeatCell({ pct }: { pct: number | null }) {
  if (pct === null) {
    return (
      <td className="border border-gray-200 py-1.5 px-2 text-center bg-gray-200 text-gray-400">
        &mdash;
      </td>
    );
  }
  return (
    <td
      className="border border-gray-200 py-1.5 px-2 text-center text-white font-bold"
      style={{ background: `hsl(${pct * 1.2}, 60%, 55%)` }}
    >
      {pct.toFixed(0)}
    </td>
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
