# Report UI Enhancement Architecture

Refactor the Report view to match upstream's component architecture and add interactive visualizations. Decided: Victory for charts (over raw SVG or Recharts), Tailwind utilities for styling (over upstream's CSS variables), and incremental implementation of 6 components starting with sortable leaderboard.

## Context

The current Report.tsx is a monolithic 228-line component with basic awards cards, totals grid, and a 7-column non-sortable leaderboard. The upstream scaffold-bench has a richer report with 13+ sortable columns, SVG charts (scatter, line), heatmaps, and a recent runs table.

We need upstream parity for the report UI to make model comparison actionable.

## Decision

**Architecture**: Refactor `Report.tsx` → `ReportPage.tsx` with sub-components matching upstream's structure (Leaderboard, AwardsGrid, CategoryHeatmap, TokenScoreScatter, ContextGrowthChart, RecentRunsTable).

**Chart library**: Victory (over raw SVG or Recharts). Victory provides React-idiomatic declarative API, good TypeScript support, and handles the chart types we need (scatter, line). Raw SVG gives maximum control but is verbose; Recharts is lighter but less flexible for custom visuals.

**Styling**: Keep Tailwind utilities, map upstream's semantic CSS variables to Tailwind equivalents (e.g., `text-text-dim` → `text-gray-500`). Upstream uses a custom theme with semantic variables; we use Tailwind's utility-first approach. Translating styles is faster and keeps our codebase consistent.

**Implementation order**: Incremental, one component at a time:
1. Sortable leaderboard (13 columns, click-to-sort, tooltips, confidence intervals)
2. Awards grid (4 cards with richer detail lines)
3. Category heatmap (per-category and per-tier breakdowns)
4. Token-score scatter (Victory scatter with Pareto frontier)
5. Context growth chart (Victory line chart with endpoint labels)
6. Recent runs table (sortable, score bars)

## Consequences

- **Bundle size**: Victory adds ~120KB gzipped. Acceptable for a benchmarking tool where users expect rich visualizations.
- **Maintenance**: Each sub-component is independently testable and replaceable. The refactor creates a cleaner architecture than the current monolith.
- **Visual fidelity**: Tailwind translation may not perfectly match upstream's pixel-perfect styling, but functional parity is the goal, not visual cloning.
