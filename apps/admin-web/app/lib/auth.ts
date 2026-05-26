// Auth token + identity helpers. Token storage uses localStorage by design
// for Phase 1 simplicity — see report for the XSS risk note. The server is
// the only authority on identity; the decoded JWT payload here is treated as
// untrusted UI hints (used to gate which sidebar links to render).

import type { Role, User } from "./types";

const ACCESS_KEY = "hb_admin_access";
const REFRESH_KEY = "hb_admin_refresh";
const USER_KEY = "hb_admin_user";
const ONBOARDING_KEY = "hb_admin_onboarding_done";
const HOTEL_KEY = "hb_admin_active_hotel";

export type StoredTokens = {
  accessToken: string;
  refreshToken: string;
  user: User;
};

function safe<T>(fn: () => T, fallback: T): T {
  if (typeof window === "undefined") return fallback;
  try {
    return fn();
  } catch {
    return fallback;
  }
}

export function setTokens(t: StoredTokens): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(ACCESS_KEY, t.accessToken);
  window.localStorage.setItem(REFRESH_KEY, t.refreshToken);
  window.localStorage.setItem(USER_KEY, JSON.stringify(t.user));
}

export function getAccessToken(): string | null {
  return safe(() => window.localStorage.getItem(ACCESS_KEY), null);
}

export function getRefreshToken(): string | null {
  return safe(() => window.localStorage.getItem(REFRESH_KEY), null);
}

export function getStoredUser(): User | null {
  return safe(() => {
    const raw = window.localStorage.getItem(USER_KEY);
    if (!raw) return null;
    return JSON.parse(raw) as User;
  }, null);
}

export function updateAccessToken(token: string): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(ACCESS_KEY, token);
}

export function updateRefreshToken(token: string): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(REFRESH_KEY, token);
}

export function clearAuth(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(ACCESS_KEY);
  window.localStorage.removeItem(REFRESH_KEY);
  window.localStorage.removeItem(USER_KEY);
  window.localStorage.removeItem(ONBOARDING_KEY);
  window.localStorage.removeItem(HOTEL_KEY);
}

export function isAuthed(): boolean {
  return getAccessToken() !== null;
}

export function markOnboardingDone(): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(ONBOARDING_KEY, "1");
}

export function isOnboardingDone(): boolean {
  return safe(() => window.localStorage.getItem(ONBOARDING_KEY) === "1", false);
}

export function setActiveHotelID(id: string): void {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(HOTEL_KEY, id);
}

export function getActiveHotelID(): string | null {
  return safe(() => window.localStorage.getItem(HOTEL_KEY), null);
}

// decodeJWT parses the payload without signature verification. Only used to
// surface role + expiry to the UI; never trust it for authorization.
export function decodeJWT(token: string): { sub?: string; role?: Role; exp?: number } | null {
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  try {
    const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload + "===".slice((payload.length + 3) % 4);
    const json = typeof atob === "function" ? atob(padded) : Buffer.from(padded, "base64").toString();
    return JSON.parse(json);
  } catch {
    return null;
  }
}
