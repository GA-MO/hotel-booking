"use client";

import Link from "next/link";
import type { Route } from "next";
import { useEffect, useState } from "react";

import {
  Anchor,
  Button,
  Card,
  Center,
  Group,
  Loader,
  ScrollArea,
  SimpleGrid,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";

import { useShell } from "@/app/components/AppShell";
import {
  EmptyState,
  ErrorBanner,
  PageHeader,
  StatusBadge,
} from "@/app/components/ui";
import { Bookings, RoomTypes, Subs } from "@/app/lib/api";
import { todayISO } from "@/app/lib/dates";
import { centsToDisplay } from "@/app/lib/money";
import { t } from "@/app/i18n";
import type { Booking, RoomType, Subscription } from "@/app/lib/types";

export default function DashboardPage() {
  const { activeHotel } = useShell();
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [sub, setSub] = useState<Subscription | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    (async () => {
      try {
        const [b, rt, s] = await Promise.all([
          Bookings.list(activeHotel.id, { limit: 100 }),
          RoomTypes.list(activeHotel.id),
          Subs.get().catch(() => null),
        ]);
        setBookings(b.bookings);
        setRoomTypes(rt.room_types);
        setSub(s);
      } catch (e) {
        setError(e instanceof Error ? e.message : t("error_generic"));
      } finally {
        setLoading(false);
      }
    })();
  }, [activeHotel.id]);

  const today = todayISO(activeHotel.timezone);

  const todayCheckIns = bookings.filter(
    (b) =>
      b.check_in_date === today &&
      (b.status === "confirmed" || b.status === "checked_in")
  );
  const occupiedToday = bookings
    .filter(
      (b) =>
        b.check_in_date <= today &&
        b.check_out_date > today &&
        (b.status === "checked_in" || b.status === "confirmed")
    )
    .reduce((acc, b) => acc + b.room_count, 0);
  const totalInventory = roomTypes.reduce((acc, r) => acc + r.total_inventory, 0);
  const occupancy =
    totalInventory > 0 ? Math.round((occupiedToday / totalInventory) * 100) : 0;

  const recent = [...bookings]
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
    .slice(0, 8);

  return (
    <div>
      <PageHeader
        title={t("nav_dashboard")}
        description={activeHotel.name}
        actions={
          <Button
            component={Link}
            href="/bookings"
            variant="default"
            size="sm"
          >
            {t("nav_bookings")}
          </Button>
        }
      />

      <ErrorBanner message={error} />

      {sub && (
        <Card withBorder radius="md" padding="sm" mb="lg">
          <Group gap="sm">
            <Text size="sm" fw={500} c="gray.7">
              {t("subscription_status")}:
            </Text>
            <StatusBadge value={sub.status} />
          </Group>
        </Card>
      )}

      <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
        <Stat title={t("today_check_ins")} value={todayCheckIns.length} />
        <Stat
          title={t("occupancy_today")}
          value={`${occupancy}%`}
          sub={`${occupiedToday} / ${totalInventory}`}
        />
        <Stat title={t("nav_room_types")} value={roomTypes.length} />
      </SimpleGrid>

      <Card withBorder radius="md" padding="lg" mt="lg">
        <Title order={3} size="h5" mb="md">
          {t("recent_bookings")}
        </Title>
        {loading ? (
          <Center py="md">
            <Loader size="sm" />
          </Center>
        ) : recent.length === 0 ? (
          <EmptyState message={t("no_data")} />
        ) : (
          <ScrollArea>
            <Table verticalSpacing="sm" highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>{t("booking_reference")}</Table.Th>
                  <Table.Th>{t("guest_name")}</Table.Th>
                  <Table.Th>{t("check_in")}</Table.Th>
                  <Table.Th>{t("status")}</Table.Th>
                  <Table.Th ta="right">{t("total")}</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {recent.map((b) => (
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
                    <Table.Td>
                      <StatusBadge value={b.status} />
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
      </Card>
    </div>
  );
}

function Stat({
  title,
  value,
  sub,
}: {
  title: string;
  value: string | number;
  sub?: string;
}) {
  return (
    <Card withBorder radius="md" padding="lg">
      <Text size="sm" c="dimmed" mb={4}>
        {title}
      </Text>
      <Text fz={28} fw={600}>
        {value}
      </Text>
      {sub && (
        <Text size="xs" c="dimmed" mt={4}>
          {sub}
        </Text>
      )}
    </Card>
  );
}
