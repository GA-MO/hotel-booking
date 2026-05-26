// Date helpers. Bookings work on calendar dates (DATE in DB), not timestamps —
// check_out is exclusive (`nights = check_out − check_in`). We deal with the
// `YYYY-MM-DD` string form everywhere and only convert to Date when we need
// to format or count nights.

const YMD = /^\d{4}-\d{2}-\d{2}$/;

export function isYMD(s: unknown): s is string {
  return typeof s === "string" && YMD.test(s);
}

// parseYMD parses a `YYYY-MM-DD` string into a Date at UTC midnight. UTC
// avoids the off-by-one trap where a Date constructed in local time near a
// DST boundary lands on the previous day.
export function parseYMD(ymd: string): Date {
  const [y, m, d] = ymd.split("-").map((p) => parseInt(p, 10));
  return new Date(Date.UTC(y, m - 1, d));
}

export function todayYMD(): string {
  const now = new Date();
  return ymd(now);
}

export function ymd(d: Date): string {
  const y = d.getUTCFullYear();
  const m = String(d.getUTCMonth() + 1).padStart(2, "0");
  const day = String(d.getUTCDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// addDays returns a new YYYY-MM-DD shifted by n days. Negative n shifts back.
export function addDays(ymdStr: string, n: number): string {
  const d = parseYMD(ymdStr);
  d.setUTCDate(d.getUTCDate() + n);
  return ymd(d);
}

// nightsBetween returns max(0, checkOut - checkIn) in whole days.
export function nightsBetween(checkIn: string, checkOut: string): number {
  if (!isYMD(checkIn) || !isYMD(checkOut)) return 0;
  const a = parseYMD(checkIn).getTime();
  const b = parseYMD(checkOut).getTime();
  const diff = Math.round((b - a) / (1000 * 60 * 60 * 24));
  return diff > 0 ? diff : 0;
}

export type FormatDateOpts = {
  locale: "th" | "en";
  // hotel.timezone (e.g. "Asia/Bangkok") — defaults to UTC to avoid leaking
  // server time into the rendered text.
  timezone?: string;
};

export function formatDate(ymdStr: string, opts: FormatDateOpts): string {
  if (!isYMD(ymdStr)) return ymdStr;
  const d = parseYMD(ymdStr);
  const intlLocale = opts.locale === "th" ? "th-TH" : "en-US";
  return new Intl.DateTimeFormat(intlLocale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    timeZone: opts.timezone ?? "UTC",
  }).format(d);
}

export function formatRange(
  checkIn: string,
  checkOut: string,
  opts: FormatDateOpts,
): string {
  return `${formatDate(checkIn, opts)} → ${formatDate(checkOut, opts)}`;
}
