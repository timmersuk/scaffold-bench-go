export function scoreTextColor(pct: number): string {
  if (pct >= 70) return "text-green-600";
  if (pct >= 40) return "text-yellow-600";
  return "text-red-600";
}

export function scoreBarColor(pct: number): string {
  if (pct >= 70) return "bg-green-500";
  if (pct >= 40) return "bg-yellow-500";
  return "bg-red-500";
}
