// Money helpers. All API money fields are int64 minor units (satang). We
// never do float math — conversion only at the display/parse boundary.

export function centsToDisplay(cents: number, currency = "THB"): string {
  const sign = cents < 0 ? "-" : "";
  const abs = Math.abs(cents);
  const major = Math.floor(abs / 100);
  const minor = abs % 100;
  const formatted =
    major.toLocaleString("en-US") + "." + minor.toString().padStart(2, "0");
  if (currency === "THB") return `฿${sign}${formatted}`;
  return `${sign}${formatted} ${currency}`;
}

// parseDisplayToCents accepts user input like "1500" or "1,500.50" and returns
// the integer satang. Returns NaN on bad input.
export function parseDisplayToCents(value: string): number {
  const clean = value.replace(/[^0-9.\-]/g, "");
  if (clean === "" || clean === "-" || clean === ".") return NaN;
  const parts = clean.split(".");
  if (parts.length > 2) return NaN;
  const major = parseInt(parts[0] || "0", 10);
  const minorRaw = (parts[1] || "00").slice(0, 2).padEnd(2, "0");
  const minor = parseInt(minorRaw, 10);
  if (Number.isNaN(major) || Number.isNaN(minor)) return NaN;
  const sign = clean.startsWith("-") ? -1 : 1;
  return sign * (Math.abs(major) * 100 + minor);
}

// rateToCents converts a base_rate decimal (NUMERIC(10,2)) to integer satang.
// Accepts the number form used by the room-type API.
export function rateToCents(rate: number): number {
  return Math.round(rate * 100);
}
