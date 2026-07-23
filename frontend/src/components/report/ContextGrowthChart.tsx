import type { ReportModelAggregate } from "../../types";
import { formatTokenCount } from "../../lib/format";
import { SectionTitle } from "./SectionTitle";

const W = 820;
const H = 360;
const PAD = { l: 64, r: 16, t: 16, b: 40 };

const PALETTE = [
  "#40a02b",
  "#1e66f5",
  "#8839ef",
  "#d20f39",
  "#e8590c",
  "#0a9396",
  "#9b59b6",
  "#b5651d",
  "#1e9e8e",
  "#c01a48",
  "#3a5a40",
  "#5b3a8c",
];

function colorFor(model: string): string {
  let h = 0;
  for (let i = 0; i < model.length; i++) h = (h * 31 + model.charCodeAt(i)) >>> 0;
  return PALETTE[h % PALETTE.length];
}

export function ContextGrowthChart({ models }: { models: ReportModelAggregate[] }) {
  const plotted = models.filter((m) => m.contextByTurn && m.contextByTurn.length > 0);

  if (plotted.length === 0) {
    return (
      <section className="mt-8">
        <SectionTitle>Context growth per turn</SectionTitle>
        <div className="text-gray-400 text-xs">No per-turn data yet (captured on new runs)</div>
      </section>
    );
  }

  const maxTurn = Math.max(...plotted.map((m) => m.contextByTurn!.length));
  const maxTokens = Math.max(
    ...plotted.flatMap((m) => m.contextByTurn!.map((p) => p.meanPromptTokens)),
    1
  );

  const plotW = W - PAD.l - PAD.r;
  const plotH = H - PAD.t - PAD.b;
  const xOf = (turn: number): number => PAD.l + ((turn - 1) / Math.max(maxTurn - 1, 1)) * plotW;
  const yOf = (tokens: number): number => PAD.t + (1 - tokens / maxTokens) * plotH;

  const xTicks = Array.from({ length: maxTurn }, (_, i) => i + 1);
  const yTicks = niceTicks(maxTokens);

  const endpoints = layoutEndpointLabels(
    plotted.map((m) => {
      const last = m.contextByTurn![m.contextByTurn!.length - 1];
      return {
        model: m.model,
        color: colorFor(m.model),
        x: xOf(last.turn),
        y: yOf(last.meanPromptTokens),
      };
    }),
    PAD.t,
    H - PAD.b
  );

  return (
    <section className="mt-8">
      <SectionTitle>Context growth per turn</SectionTitle>
      <div className="text-[11px] text-gray-500 mb-2">
        Mean prompt tokens fed at each turn index &mdash; flat = tight working set, rising = re-feeds
        everything.
      </div>
      <div className="overflow-x-auto">
        <svg
          role="img"
          aria-label="Context growth per turn, per model"
          viewBox={`0 0 ${W} ${H}`}
          className="w-full max-w-[820px]"
          style={{ minHeight: 300 }}
        >
          {xTicks.map((t) => {
            const x = xOf(t);
            return (
              <g key={`x${t}`}>
                <line
                  x1={x}
                  x2={x}
                  y1={PAD.t}
                  y2={H - PAD.b}
                  stroke="#e5e7eb"
                  strokeWidth={1}
                />
                <text
                  x={x}
                  y={H - PAD.b + 16}
                  textAnchor="middle"
                  fontSize={10}
                  fill="#6b7280"
                >
                  {t}
                </text>
              </g>
            );
          })}
          {yTicks.map((t) => {
            const y = yOf(t);
            return (
              <g key={`y${t}`}>
                <line
                  x1={PAD.l}
                  x2={W - PAD.r}
                  y1={y}
                  y2={y}
                  stroke="#e5e7eb"
                  strokeWidth={1}
                />
                <text
                  x={PAD.l - 8}
                  y={y + 3}
                  textAnchor="end"
                  fontSize={10}
                  fill="#6b7280"
                >
                  {formatTokenCount(t)}
                </text>
              </g>
            );
          })}
          <line
            x1={PAD.l}
            x2={PAD.l}
            y1={PAD.t}
            y2={H - PAD.b}
            stroke="#111827"
            strokeWidth={1.5}
          />
          <line
            x1={PAD.l}
            x2={W - PAD.r}
            y1={H - PAD.b}
            y2={H - PAD.b}
            stroke="#111827"
            strokeWidth={1.5}
          />
          <text
            x={(PAD.l + W - PAD.r) / 2}
            y={H - 4}
            textAnchor="middle"
            fontSize={11}
            fill="#6b7280"
          >
            turn index (request order)
          </text>
          <text
            x={-(H - PAD.b) / 2 - PAD.t / 2}
            y={16}
            textAnchor="middle"
            fontSize={11}
            fill="#6b7280"
            transform="rotate(-90)"
          >
            prompt tokens (mean)
          </text>

          {plotted.map((m) => {
            const c = colorFor(m.model);
            const pts = m.contextByTurn!;
            const d = pts
              .map(
                (p, i) =>
                  `${i === 0 ? "M" : "L"} ${xOf(p.turn).toFixed(1)} ${yOf(p.meanPromptTokens).toFixed(1)}`
              )
              .join(" ");
            return (
              <g key={m.model}>
                <path d={d} fill="none" stroke={c} strokeWidth={1.75} />
                {pts.map((p) => {
                  const x = xOf(p.turn);
                  const y = yOf(p.meanPromptTokens);
                  const opacity = Math.max(0.25, Math.min(1, p.runs / 3));
                  return (
                    <circle key={p.turn} cx={x} cy={y} r={2.5} fill={c} opacity={opacity}>
                      <title>{`${m.model} \u00b7 turn ${p.turn} \u00b7 ${formatTokenCount(p.meanPromptTokens)} tok \u00b7 n=${p.runs}`}</title>
                    </circle>
                  );
                })}
              </g>
            );
          })}

          {endpoints.map((e) => {
            const right = e.side === "right";
            const tx = right ? e.x + 7 : e.x - 7;
            return (
              <g key={`lbl-${e.model}`}>
                {Math.abs(e.labelY - e.y) > 1 && (
                  <line
                    x1={e.x}
                    y1={e.y}
                    x2={tx}
                    y2={e.labelY}
                    stroke={e.color}
                    strokeWidth={0.75}
                    opacity={0.5}
                  />
                )}
                <text
                  x={tx}
                  y={e.labelY + 3}
                  textAnchor={right ? "start" : "end"}
                  fontSize={10}
                  fontWeight={700}
                  fill={e.color}
                >
                  {e.model}
                </text>
              </g>
            );
          })}
        </svg>
      </div>
    </section>
  );
}

type EndpointLabel = {
  model: string;
  color: string;
  x: number;
  y: number;
  side: "left" | "right";
  labelY: number;
};

function layoutEndpointLabels(
  items: Array<{ model: string; color: string; x: number; y: number }>,
  minY: number,
  maxY: number
): EndpointLabel[] {
  const MIN_GAP = 12;
  const rightThreshold = W - 140;
  const withSide: EndpointLabel[] = items.map((it) => ({
    ...it,
    side: it.x > rightThreshold ? "left" : "right",
    labelY: it.y,
  }));
  for (const side of ["left", "right"] as const) {
    const group = withSide.filter((i) => i.side === side).sort((a, b) => a.labelY - b.labelY);
    for (let i = 1; i < group.length; i++) {
      if (group[i].labelY - group[i - 1].labelY < MIN_GAP)
        group[i].labelY = group[i - 1].labelY + MIN_GAP;
    }
    const overflow = group.length ? group[group.length - 1].labelY - maxY : 0;
    if (overflow > 0) for (const g of group) g.labelY = Math.max(minY, g.labelY - overflow);
  }
  return withSide;
}

function niceTicks(max: number): number[] {
  const step = niceStep(max / 4);
  const ticks: number[] = [];
  for (let v = 0; v <= max; v += step) ticks.push(v);
  if (ticks.length === 0) ticks.push(0, Math.round(max));
  return ticks;
}

function niceStep(raw: number): number {
  if (raw <= 0) return 1;
  const pow = Math.pow(10, Math.floor(Math.log10(raw)));
  const n = raw / pow;
  const nice = n < 1.5 ? 1 : n < 3 ? 2 : n < 7 ? 5 : 10;
  return nice * pow;
}
