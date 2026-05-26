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

import {
  AppShell as MantineAppShell,
  Burger,
  Center,
  Group,
  Loader,
  NavLink as MantineNavLink,
  ScrollArea,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";

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
  const [mobileOpened, { toggle: toggleMobile, close: closeMobile }] =
    useDisclosure();
  const [error, setError] = useState<string | null>(null);
  // Read storage after mount only — avoids the hydration mismatch we hit
  // when getStoredUser() ran during the initial render.
  const [userEmail, setUserEmail] = useState<string | null>(null);

  useEffect(() => {
    const u = getStoredUser();
    setUserEmail(u?.email || null);
  }, []);

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
      <Center mih="100vh">
        <Group gap="sm">
          <Loader size="sm" />
          <Text size="sm" c="dimmed">
            {t("loading")}
          </Text>
        </Group>
      </Center>
    );
  }

  if (hotels.length === 0) {
    return (
      <Center mih="100vh" p="md">
        <Stack align="center" gap="md">
          <Text size="sm" c="dimmed">
            {error || t("no_data")}
          </Text>
          <Link
            href="/onboarding"
            className="rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white"
          >
            {t("onboarding_title")}
          </Link>
        </Stack>
      </Center>
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
      <MantineAppShell
        header={{ height: 56 }}
        navbar={{
          width: 240,
          breakpoint: "sm",
          collapsed: { mobile: !mobileOpened },
        }}
        padding="lg"
      >
        <MantineAppShell.Header>
          <Group h="100%" px="md" justify="space-between">
            <Group gap="sm">
              <Burger
                opened={mobileOpened}
                onClick={toggleMobile}
                hiddenFrom="sm"
                size="sm"
                aria-label="Toggle navigation"
              />
              <Text fw={600} size="sm">
                {t("app_title")}
              </Text>
            </Group>
            <Link
              href="/logout"
              className="text-sm text-neutral-600 hover:text-neutral-900"
            >
              {t("log_out")}
            </Link>
          </Group>
        </MantineAppShell.Header>

        <MantineAppShell.Navbar p="md">
          <MantineAppShell.Section>
            <Stack gap={2} mb="sm">
              <Text fw={600} size="sm">
                {t("app_title")}
              </Text>
              {userEmail && (
                <Text size="xs" c="dimmed" truncate>
                  {userEmail}
                </Text>
              )}
            </Stack>
            {hotels.length > 1 ? (
              <Select
                size="xs"
                value={activeHotel.id}
                data={hotels.map((h) => ({ value: h.id, label: h.name }))}
                onChange={(val) => val && ctx.setActive(val)}
                allowDeselect={false}
                mb="sm"
              />
            ) : (
              <Text size="xs" fw={500} c="gray.7" mb="sm" truncate>
                {activeHotel.name}
              </Text>
            )}
          </MantineAppShell.Section>

          <MantineAppShell.Section grow component={ScrollArea}>
            {NAV.map((item) => {
              const active =
                item.href === "/"
                  ? pathname === "/"
                  : pathname?.startsWith(item.href) ?? false;
              return (
                <MantineNavLink
                  key={item.href}
                  component={Link}
                  href={item.href}
                  label={t(item.labelKey)}
                  active={active}
                  onClick={closeMobile}
                  variant={active ? "filled" : "subtle"}
                  color="dark"
                  mb={2}
                />
              );
            })}
          </MantineAppShell.Section>

          <MantineAppShell.Section>
            <MantineNavLink
              component={Link}
              href={"/logout" as Route}
              label={t("nav_logout")}
              variant="subtle"
              color="gray"
            />
          </MantineAppShell.Section>
        </MantineAppShell.Navbar>

        <MantineAppShell.Main>{children}</MantineAppShell.Main>
      </MantineAppShell>
    </ShellCtx.Provider>
  );
}
