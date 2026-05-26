"use client";

import Link from "next/link";
import type { Route } from "next";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import {
  Anchor,
  Center,
  Group,
  Loader,
  NativeSelect,
  ScrollArea,
  SegmentedControl,
  Table,
  Text,
  TextInput,
} from "@mantine/core";
import { MonthView, type ScheduleEventData } from "@mantine/schedule";

import { useShell } from "@/app/components/AppShell";
import {
  EmptyState,
  ErrorBanner,
  PageHeader,
  StatusBadge,
} from "@/app/components/ui";
import { Bookings } from "@/app/lib/api";
import { centsToDisplay } from "@/app/lib/money";
import { t } from "@/app/i18n";
import type { Booking, BookingStatus } from "@/app/lib/types";

const STATUSES: BookingStatus[] = [
  "pending_payment",
  "confirmed",
  "cancelled",
  "expired",
  "checked_in",
  "checked_out",
  "no_show",
  "completed",
];

const STATUS_OPTIONS = [
  { value: "", label: t("all") },
  ...STATUSES.map((s) => ({ value: s, label: s.replace(/_/g, " ") })),
];

// Mantine palette name per booking status — keeps the calendar's event-bar
// color in sync with StatusBadge so the two views read the same.
const STATUS_TO_COLOR: Record<string, string> = {
  pending_payment: "yellow",
  confirmed: "teal",
  cancelled: "red",
  expired: "gray",
  checked_in: "blue",
  checked_out: "gray",
  no_show: "red",
  completed: "teal",
};

type ViewMode = "list" | "calendar";

export default function BookingsListPage() {
  const router = useRouter();
  const { activeHotel } = useShell();
  const [items, setItems] = useState<Booking[]>([]);
  const [status, setStatus] = useState<string>("");
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");
  const [view, setView] = useState<ViewMode>("list");
  // MonthView uses "YYYY-MM-DD" string dates; default to today in hotel TZ.
  const [calendarDate, setCalendarDate] = useState<string>(
    () => new Date().toISOString().slice(0, 10)
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    Bookings.list(activeHotel.id, status ? { status, limit: 200 } : { limit: 200 })
      .then((r) => setItems(r.bookings))
      .catch((e) => setError(e instanceof Error ? e.message : t("error_generic")))
      .finally(() => setLoading(false));
  }, [activeHotel.id, status]);

  const filtered = useMemo(() => {
    return items.filter((b) => {
      if (from && b.check_out_date <= from) return false;
      if (to && b.check_in_date >= to) return false;
      return true;
    });
  }, [items, from, to]);

  // Map bookings to ScheduleEventData. start/end take "YYYY-MM-DD HH:mm:ss"
  // strings; we treat each booking as a multi-day all-day event spanning
  // check-in (inclusive) to check-out (exclusive, last night is the day
  // before check-out per hotel convention).
  const events = useMemo<ScheduleEventData[]>(() => {
    return filtered.map((b) => ({
      id: b.id,
      title: `${b.guest_name} · ${b.room_count}r · ${b.reference}`,
      start: `${b.check_in_date} 00:00:00`,
      end: `${b.check_out_date} 00:00:00`,
      color: STATUS_TO_COLOR[b.status] || "gray",
    }));
  }, [filtered]);

  return (
    <div>
      <PageHeader
        title={t("nav_bookings")}
        description={activeHotel.name}
        actions={
          <SegmentedControl
            value={view}
            onChange={(v) => setView(v as ViewMode)}
            data={[
              { value: "list", label: t("view_list") },
              { value: "calendar", label: t("view_calendar") },
            ]}
            size="sm"
          />
        }
      />

      {view === "list" && (
        <Group grow mb="md" wrap="wrap" align="end">
          <NativeSelect
            label={t("filter_status")}
            data={STATUS_OPTIONS}
            value={status}
            onChange={(e) => setStatus(e.currentTarget.value)}
          />
          <TextInput
            label={t("check_in") + " ≥"}
            type="date"
            value={from}
            onChange={(e) => setFrom(e.currentTarget.value)}
          />
          <TextInput
            label={t("check_out") + " ≤"}
            type="date"
            value={to}
            onChange={(e) => setTo(e.currentTarget.value)}
          />
        </Group>
      )}

      <ErrorBanner message={error} />

      {loading ? (
        <Center py="xl">
          <Group gap="sm">
            <Loader size="sm" />
            <Text size="sm" c="dimmed">
              {t("loading")}
            </Text>
          </Group>
        </Center>
      ) : view === "calendar" ? (
        // Default mode keeps onEventClick working. We don't wire onEventDrop /
        // onDrop yet — without those callbacks the drag still works visually
        // but the change isn't persisted; once BE has a reschedule endpoint
        // we can opt in by adding the handler.
        <MonthView
          date={calendarDate}
          onDateChange={setCalendarDate}
          events={events}
          onEventClick={(e) =>
            router.push(`/bookings/${e.id}` as Route)
          }
        />
      ) : filtered.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <ScrollArea>
          <Table
            highlightOnHover
            striped="even"
            withTableBorder
            withRowBorders
            verticalSpacing="sm"
            miw={800}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th>{t("booking_reference")}</Table.Th>
                <Table.Th>{t("guest_name")}</Table.Th>
                <Table.Th>{t("check_in")}</Table.Th>
                <Table.Th>{t("check_out")}</Table.Th>
                <Table.Th>{t("status")}</Table.Th>
                <Table.Th>{t("source")}</Table.Th>
                <Table.Th ta="right">{t("total")}</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {filtered.map((b) => (
                <Table.Tr key={b.id}>
                  <Table.Td>
                    <Anchor
                      component={Link}
                      href={`/bookings/${b.id}` as Route}
                      ff="monospace"
                      size="xs"
                      c="dark"
                    >
                      {b.reference}
                    </Anchor>
                  </Table.Td>
                  <Table.Td>{b.guest_name}</Table.Td>
                  <Table.Td>{b.check_in_date}</Table.Td>
                  <Table.Td>{b.check_out_date}</Table.Td>
                  <Table.Td>
                    <StatusBadge value={b.status} />
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs" c="dimmed">
                      {b.source}
                    </Text>
                  </Table.Td>
                  <Table.Td ta="right" ff="monospace">
                    {centsToDisplay(b.total_cents, b.currency)}
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      )}
    </div>
  );
}
