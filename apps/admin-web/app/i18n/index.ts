// Tiny i18n dictionary. TH default, EN fallback. No library — Phase 1 UI
// strings are stable enough not to need ICU plural rules.

import { th } from "./th";
import { en } from "./en";

export type Dict = typeof th;
export type Locale = "th" | "en";

const dicts: Record<Locale, Dict> = { th, en };
const LOCALE_KEY = "hb_admin_locale";

export function getLocale(): Locale {
  if (typeof window === "undefined") return "th";
  const stored = window.localStorage.getItem(LOCALE_KEY);
  if (stored === "en" || stored === "th") return stored;
  return "th";
}

export function setLocale(l: Locale): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(LOCALE_KEY, l);
}

export function t(key: keyof Dict, locale?: Locale): string {
  const l = locale || getLocale();
  return dicts[l][key] || dicts.en[key] || String(key);
}
