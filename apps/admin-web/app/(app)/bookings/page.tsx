"use client";

import Link from "next/link";
import type { Route } from "next";
import { useEffect, useMemo, useState } from "react";

import {
  Anchor,
  Center,
  Group,
  Loader,
  NativeSelect,
  ScrollArea,
  Table,
  Text,
  TextInput,
} from "@mantine/core";

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

export default function BookingsListPage() {
  const { activeHotel } = useShell();
  const [items, setItems] = useState<Booking[]>([]);
  const [status, setStatus] = useState<string>("");
  const [from, setFrom] = useState<string>("");
  const [to, setTo] = useState<string>("");
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

  return (
    <div>
      <PageHeader title={t("nav_bookings")} description={activeHotel.name} />

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
