import type { ReportModelAggregate } from "../../types";
import { SectionTitle } from "./SectionTitle";

export function MetricBars({
  title,
  models,
  value,
  format,
  color,
  lowerIsBetter = false,
}: {
  title: string;
  models: ReportModelAggregate[];
  value: (model: ReportModelAggregate) => number | null;
  format: (value: number, model: ReportModelAggregate) => string;
  color: string;
  lowerIsBetter?: boolean;
}) {
  const rows = models.filter((model) => value(model) !== null);
  const values = rows.map((model) => value(model) ?? 0);
  const max = Math.max(...values, 0);
  const min = Math.min(...values, max);

  return (
    <section className="mt-8">
      <SectionTitle>{title}</SectionTitle>
      {rows.length === 0 ? (
        <div className="text-gray-400 text-xs">No data</div>
      ) : (
        <div className="grid grid-cols-[minmax(160px,260px)_1fr_80px] gap-y-1 items-center text-xs">
          {rows.map((model) => {
            const metric = value(model) ?? 0;
            const pct = barPct(metric, min, max, lowerIsBetter);
            return (
              <MetricBarRow
                key={model.model}
                model={model}
                pct={pct}
                value={format(metric, model)}
                color={color}
              />
            );
          })}
        </div>
      )}
    </section>
  );
}

function MetricBarRow({
  model,
  pct,
  value,
  color,
}: {
  model: ReportModelAggregate;
  pct: number;
  value: string;
  color: string;
}) {
  return (
    <>
      <div className="flex items-center gap-2 min-w-0 pr-3">
        <span className="truncate text-gray-900">{model.model}</span>
        <SourceBadge source={model.source} />
      </div>
      <div className="h-2.5 bg-white rounded-sm overflow-hidden border border-gray-200">
        <div className="h-full" style={{ width: `${pct}%`, background: color }} />
      </div>
      <div className="text-right text-gray-500 tabular-nums">{value}</div>
    </>
  );
}

function barPct(value: number, min: number, max: number, lowerIsBetter: boolean): number {
  if (max === min) return 100;
  return lowerIsBetter ? ((max - value) / (max - min)) * 95 + 5 : (value / max) * 100;
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
