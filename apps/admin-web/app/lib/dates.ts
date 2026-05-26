// Date helpers. The API stores `*_date` columns as DATE (YYYY-MM-DD) and
// timestamps in UTC, but displays in the hotel timezone. Phase 1 keeps it
// simple: render dates in the hotel TZ using Intl.

export function todayISO(timezone?: string): string {
  const now = new Date();
  if (!timezone) return now.toISOString().slice(0, 10);
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: timezone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(now);
  const y = parts.find((p) => p.type === "year")?.value;
  const m = parts.find((p) => p.type === "month")?.value;
  const d = parts.find((p) => p.type === "day")?.value;
  return `${y}-${m}-${d}`;
}

export function addDaysISO(iso: string, days: number): string {
  const [y, m, d] = iso.split("-").map((n) => parseInt(n, 10));
  const date = new Date(Date.UTC(y, m - 1, d));
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

export function daysBetween(startISO: string, endISO: string): number {
  const a = isoToUTC(startISO);
  const b = isoToUTC(endISO);
  return Math.round((b.getTime() - a.getTime()) / 86_400_000);
}

function isoToUTC(iso: string): Date {
  const [y, m, d] = iso.split("-").map((n) => parseInt(n, 10));
  return new Date(Date.UTC(y, m - 1, d));
}

export function formatDate(iso: string, locale = "en-GB"): string {
  if (!iso) return "";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "2-digit",
  }).format(new Date(iso));
}

export function formatDateTime(ts: string, timezone?: string, locale = "en-GB"): string {
  if (!ts) return "";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: timezone,
  }).format(new Date(ts));
}

export function monthRange(year: number, monthZeroBased: number): { start: string; end: string; days: string[] } {
  const start = new Date(Date.UTC(year, monthZeroBased, 1));
  const end = new Date(Date.UTC(year, monthZeroBased + 1, 1));
  const days: string[] = [];
  for (let d = new Date(start); d < end; d.setUTCDate(d.getUTCDate() + 1)) {
    days.push(d.toISOString().slice(0, 10));
  }
  return {
    start: start.toISOString().slice(0, 10),
    end: end.toISOString().slice(0, 10),
    days,
  };
}
