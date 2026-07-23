export function formatElapsed(ms: number): string {
  const total = Math.floor(ms / 1000);
  const m = Math.floor(total / 60)
    .toString()
    .padStart(2, "0");
  const s = (total % 60).toString().padStart(2, "0");
  return `${m}:${s}`;
}

export function formatWallTime(seconds: number): string {
  const total = Math.floor(seconds);
  if (total < 60) return `${total}s`;
  const hours = Math.floor(total / 3600);
  const mins = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m ${secs.toString().padStart(2, "0")}s`;
}

export function formatNowHHMMSS(): string {
  return new Date().toISOString().substring(11, 19);
}

export function formatTokenCount(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0";
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return `${Math.round(n)}`;
}

export function formatTps(value: number | null, approx: boolean, digits: number): string {
  if (value === null) return "\u2014";
  return `${approx ? "~" : ""}${value.toFixed(digits)}`;
}

export function formatSeconds(value: number | null, digits: number): string {
  return value === null ? "\u2014" : `${value.toFixed(digits)}s`;
}
