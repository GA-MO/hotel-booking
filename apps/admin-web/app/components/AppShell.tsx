"use client";

import Link from "next/link";
import type { Route } from "next";
import { usePathname, useRouter } from "next/navigation";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";

import { Hotels } from "@/app/lib/api";
import {
  getActiveHotelID,
  getStoredUser,
  isAuthed,
  isOnboardingDone,
  markOnboardingDone,
  setActiveHotelID,
} from "@/app/lib/auth";
import { t } from "@/app/i18n";
import type { Hotel } from "@/app/lib/types";

type NavItem = { href: Route; labelKey: Parameters<typeof t>[0] };

const NAV: NavItem[] = [
  { href: "/" as Route, labelKey: "nav_dashboard" },
  { href: "/calendar" as Route, labelKey: "nav_calendar" },
  { href: "/bookings" as Route, labelKey: "nav_bookings" },
  { href: "/hotel" as Route, labelKey: "nav_hotel" },
  { href: "/room-types" as Route, labelKey: "nav_room_types" },
  { href: "/landing" as Route, labelKey: "nav_landing" },
  { href: "/pricing" as Route, labelKey: "nav_pricing" },
  { href: "/settings" as Route, labelKey: "nav_settings" },
];

type ShellContext = {
  hotels: Hotel[];
  activeHotel: Hotel;
  setActive: (id: string) => void;
  reloadHotels: () => Promise<void>;
};

const ShellCtx = createContext<ShellContext | null>(null);

export function useShell(): ShellContext {
  const v = useContext(ShellCtx);
  if (!v) throw new Error("useShell outside AppShell");
  return v;
}

export function AppShell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const pathname = usePathname();
  const [hotels, setHotels] = useState<Hotel[] | null>(null);
  const [activeID, setActiveID] = useState<string | null>(null);
  const [navOpen, setNavOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const user = typeof window !== "undefined" ? getStoredUser() : null;

  const load = useCallback(async () => {
    try {
      const res = await Hotels.list();
      setHotels(res.hotels);
      const stored = getActiveHotelID();
      const validStored = res.hotels.find((h) => h.id === stored);
      const chosen = validStored?.id || res.hotels[0]?.id || null;
      if (chosen) {
        setActiveID(chosen);
        setActiveHotelID(chosen);
      } else {
        setActiveID(null);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
      setHotels([]);
    }
  }, []);

  useEffect(() => {
    if (!isAuthed()) {
      router.replace("/login");
      return;
    }
    void load();
  }, [router, load]);

  useEffect(() => {
    if (hotels === null) return;
    if (hotels.length === 0 && !isOnboardingDone() && pathname !== "/onboarding") {
      router.replace("/onboarding");
      return;
    }
    if (hotels.length > 0 && !isOnboardingDone()) {
      markOnboardingDone();
    }
  }, [hotels, pathname, router]);

  if (hotels === null) {
    return (
      <main className="flex min-h-screen items-center justify-center text-sm text-neutral-500">
        {t("loading")}
      </main>
    );
  }

  if (hotels.length === 0) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4 text-center text-sm text-neutral-600">
        <div>
          <p>{error || t("no_data")}</p>
          <Link
            href="/onboarding"
            className="mt-3 inline-block rounded-md bg-neutral-900 px-4 py-2 text-white"
          >
            {t("onboarding_title")}
          </Link>
        </div>
      </main>
    );
  }

  const activeHotel = hotels.find((h) => h.id === activeID) || hotels[0];

  const ctx: ShellContext = {
    hotels,
    activeHotel,
    setActive: (id: string) => {
      setActiveID(id);
      setActiveHotelID(id);
    },
    reloadHotels: load,
  };

  return (
    <ShellCtx.Provider value={ctx}>
      <div className="min-h-screen bg-neutral-50 text-neutral-900">
        <header className="sticky top-0 z-20 border-b border-neutral-200 bg-white px-4 py-2.5 md:hidden">
          <div className="flex items-center justify-between">
            <button
              onClick={() => setNavOpen((v) => !v)}
              className="rounded-md border border-neutral-300 px-2.5 py-1 text-sm"
              aria-label="Menu"
            >
              ☰
            </button>
            <span className="font-semibold">{t("app_title")}</span>
            <Link href="/logout" className="text-sm text-neutral-600">
              {t("log_out")}
            </Link>
          </div>
        </header>

        <div className="md:flex">
          <aside
            className={`${navOpen ? "block" : "hidden"} border-b border-neutral-200 bg-white md:sticky md:top-0 md:block md:h-screen md:w-60 md:flex-shrink-0 md:border-b-0 md:border-r`}
          >
            <div className="px-5 py-5">
              <div className="text-sm font-semibold text-neutral-900">
                {t("app_title")}
              </div>
              {user && (
                <div className="mt-0.5 truncate text-xs text-neutral-500">
                  {user.email}
                </div>
              )}
            </div>

            {hotels.length > 1 ? (
              <div className="px-5 pb-3">
                <select
                  value={activeHotel.id}
                  onChange={(e) => ctx.setActive(e.target.value)}
                  className="w-full rounded-md border border-neutral-300 bg-white px-2 py-1.5 text-xs"
                >
                  {hotels.map((h) => (
                    <option key={h.id} value={h.id}>
                      {h.name}
                    </option>
                  ))}
                </select>
              </div>
            ) : (
              <div className="truncate px-5 pb-3 text-xs font-medium text-neutral-700">
                {activeHotel.name}
              </div>
            )}

            <nav className="px-2 pb-6">
              {NAV.map((item) => {
                const active =
                  item.href === "/"
                    ? pathname === "/"
                    : pathname?.startsWith(item.href);
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setNavOpen(false)}
                    className={`mb-0.5 block rounded-md px-3 py-2 text-sm ${active ? "bg-neutral-900 text-white" : "text-neutral-700 hover:bg-neutral-100"}`}
                  >
                    {t(item.labelKey)}
                  </Link>
                );
              })}
              <Link
                href="/logout"
                className="mt-4 block rounded-md px-3 py-2 text-sm text-neutral-500 hover:bg-neutral-100"
              >
                {t("nav_logout")}
              </Link>
            </nav>
          </aside>

          <main className="min-w-0 flex-1 px-4 py-6 md:px-8 md:py-8">
            {children}
          </main>
        </div>
      </div>
    </ShellCtx.Provider>
  );
}

