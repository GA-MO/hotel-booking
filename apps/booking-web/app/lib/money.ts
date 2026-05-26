// Money helpers. Backend stores integer minor units in `*_cents` fields and
// also returns decimal strings ("1500.00") for the quote endpoint. We never
// parse the decimal-string form back to a number for math — pricing math
// happens server-side; the FE only formats for display.

// minorUnitsFromCurrency returns the smallest-unit divisor for an ISO 4217
// code. Phase 1 covers THB / USD / EUR / GBP / JPY (the last has no minor
// units). Anything else falls back to 100 — fine for display rounding.
function minorUnitsFromCurrency(currency: string): number {
  switch (currency.toUpperCase()) {
    case "JPY":
    case "KRW":
    case "VND":
      return 1;
    default:
      return 100;
  }
}

export function formatMoney(amountCents: number, currency: string, locale = "en-US"): string {
  const divisor = minorUnitsFromCurrency(currency);
  const value = amountCents / divisor;
  try {
    return new Intl.NumberFormat(locale, {
      style: "currency",
      currency: currency.toUpperCase(),
      minimumFractionDigits: divisor === 1 ? 0 : 2,
      maximumFractionDigits: divisor === 1 ? 0 : 2,
    }).format(value);
  } catch {
    // Some currency codes are unknown to older runtimes; fall back to plain
    // formatting prefixed with the code.
    return `${currency.toUpperCase()} ${value.toFixed(divisor === 1 ? 0 : 2)}`;
  }
}

// formatDecimal renders a decimal string (e.g. "1500.00") returned from the
// quote endpoint. We pass the raw string through Intl by converting via
// Number — safe for display only, never for math.
export function formatDecimal(amount: string, currency: string, locale = "en-US"): string {
  const n = Number(amount);
  if (!Number.isFinite(n)) return `${currency.toUpperCase()} ${amount}`;
  const divisor = minorUnitsFromCurrency(currency);
  try {
    return new Intl.NumberFormat(locale, {
      style: "currency",
      currency: currency.toUpperCase(),
      minimumFractionDigits: divisor === 1 ? 0 : 2,
      maximumFractionDigits: divisor === 1 ? 0 : 2,
    }).format(n);
  } catch {
    return `${currency.toUpperCase()} ${amount}`;
  }
}

export function localeFor(lang: "th" | "en"): string {
  return lang === "th" ? "th-TH" : "en-US";
}
