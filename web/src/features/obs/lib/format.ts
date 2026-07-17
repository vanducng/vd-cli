export const pad2 = (n: number) => String(n).padStart(2, "0");

/** HH:MM:SS — turn header clock. */
export function formatClock(d: Date): string {
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`;
}

/** MM-DD HH:MM — sessions table started column. Locale-independent by design. */
export function formatStarted(d: Date): string {
  return `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
}

/** Turn duration: "45s" or "2m05s". */
export function formatDuration(ms: number): string {
  const totalSec = Math.round(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  return m === 0 ? `${s}s` : `${m}m${pad2(s)}s`;
}

/** Tool-span duration, sub-second granularity: "820ms" or "1.4s". */
export function formatMs(ms: number): string {
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`;
}

/** "$1.23" — a bare positive cost. Nil-cost rendering lives in <CostCell>. */
export function formatUsd(v: number): string {
  return `$${v.toFixed(2)}`;
}

/** "2,921" — thousands-separated, locale-independent enough for a counter. */
export function formatCount(n: number): string {
  return n.toLocaleString("en-US");
}

/** "1.34%" — a 0..1 fraction as a percent. */
export function formatPct(v: number): string {
  return `${(v * 100).toFixed(2)}%`;
}
