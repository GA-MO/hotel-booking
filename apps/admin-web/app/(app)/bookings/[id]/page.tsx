"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import {
  Anchor,
  Button,
  Card,
  Center,
  Divider,
  Group,
  Loader,
  Modal,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  Timeline,
  Title,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";

import { useShell } from "@/app/components/AppShell";
import {
  ErrorBanner,
  PageHeader,
  StatusBadge,
} from "@/app/components/ui";
import { Bookings, ApiClientError } from "@/app/lib/api";
import { formatDateTime } from "@/app/lib/dates";
import { centsToDisplay } from "@/app/lib/money";
import { notifySuccess } from "@/app/lib/notify";
import { t, type Dict } from "@/app/i18n";
import type { Booking, BookingEvent } from "@/app/lib/types";

type ActionKey = "confirm" | "checkIn" | "checkOut" | "cancel" | "noShow";

const eventLabelKey: Record<string, string> = {
  created: "booking_event_created",
  confirmed: "booking_event_confirmed",
  cancelled: "booking_event_cancelled",
  checked_in: "booking_event_checked_in",
  checked_out: "booking_event_checked_out",
  no_show: "booking_event_no_show",
  expired: "booking_event_expired",
  payment_claimed: "booking_event_payment_claimed",
};

const actionLabelKey: Record<ActionKey, keyof Dict> = {
  confirm: "action_confirm",
  checkIn: "action_check_in",
  checkOut: "action_check_out",
  noShow: "action_no_show",
  cancel: "action_cancel",
};

export default function BookingDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { activeHotel } = useShell();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [events, setEvents] = useState<BookingEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<ActionKey | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [cancelOpened, cancelModal] = useDisclosure(false);
  const [cancelReason, setCancelReason] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [b, ev] = await Promise.all([
        Bookings.get(activeHotel.id, params.id),
        Bookings.listEvents(activeHotel.id, params.id).catch(() => ({ events: [] })),
      ]);
      setBooking(b);
      setEvents(ev.events);
    } catch (e) {
      if (e instanceof ApiClientError && e.status === 404) {
        router.replace("/bookings");
        return;
      }
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setLoading(false);
    }
  }, [activeHotel.id, params.id, router]);

  useEffect(() => {
    void load();
  }, [load]);

  async function runAction(key: ActionKey, reason?: string) {
    if (!booking) return;
    setActionLoading(key);
    setError(null);
    try {
      let updated: Booking;
      switch (key) {
        case "confirm":
          updated = await Bookings.confirm(activeHotel.id, booking.id);
          break;
        case "checkIn":
          updated = await Bookings.checkIn(activeHotel.id, booking.id);
          break;
        case "checkOut":
          updated = await Bookings.checkOut(activeHotel.id, booking.id);
          break;
        case "noShow":
          updated = await Bookings.noShow(activeHotel.id, booking.id);
          break;
        case "cancel":
          updated = await Bookings.cancel(
            activeHotel.id,
            booking.id,
            reason || ""
          );
          break;
      }
      setBooking(updated);
      notifySuccess(t(actionLabelKey[key]) + " ✓");
      try {
        const refreshed = await Bookings.listEvents(activeHotel.id, booking.id);
        setEvents(refreshed.events);
      } catch {
        // events refresh is best-effort
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setActionLoading(null);
    }
  }

  if (loading) {
    return (
      <Center py="xl">
        <Group gap="sm">
          <Loader size="sm" />
          <Text size="sm" c="dimmed">
            {t("loading")}
          </Text>
        </Group>
      </Center>
    );
  }
  if (!booking) {
    return <ErrorBanner message={error || "Not found"} />;
  }

  const canConfirm = booking.status === "pending_payment";
  const canCheckIn = booking.status === "confirmed";
  const canCheckOut = booking.status === "checked_in";
  const canCancel = ["pending_payment", "confirmed"].includes(booking.status);
  const canNoShow = booking.status === "confirmed";

  // Synthesized fallback (from booking row timestamps) only when the audit
  // endpoint returns no rows — keeps the timeline useful for pre-migration data.
  const fallbackTimeline = [
    { label: "created", at: booking.created_at },
    { label: "confirmed", at: booking.confirmed_at },
    { label: "checked in", at: booking.checked_in_at },
    { label: "checked out", at: booking.checked_out_at },
    { label: "cancelled", at: booking.cancelled_at },
  ].filter((e) => e.at) as Array<{ label: string; at: string }>;

  return (
    <div>
      <PageHeader
        title={booking.reference}
        description={`${booking.guest_name} — ${activeHotel.name}`}
        actions={
          <Button
            component={Link}
            href="/bookings"
            variant="default"
            size="sm"
          >
            ← {t("nav_bookings")}
          </Button>
        }
      />

      <ErrorBanner message={error} />

      <Group mb="md" wrap="wrap">
        {canConfirm && (
          <Button
            color="dark"
            onClick={() => runAction("confirm")}
            loading={actionLoading === "confirm"}
          >
            {t("action_confirm")}
          </Button>
        )}
        {canCheckIn && (
          <Button
            color="dark"
            onClick={() => runAction("checkIn")}
            loading={actionLoading === "checkIn"}
          >
            {t("action_check_in")}
          </Button>
        )}
        {canCheckOut && (
          <Button
            color="dark"
            onClick={() => runAction("checkOut")}
            loading={actionLoading === "checkOut"}
          >
            {t("action_check_out")}
          </Button>
        )}
        {canNoShow && (
          <Button
            variant="default"
            onClick={() => runAction("noShow")}
            loading={actionLoading === "noShow"}
          >
            {t("action_no_show")}
          </Button>
        )}
        {canCancel && (
          <Button
            color="red"
            variant="light"
            onClick={() => {
              setCancelReason("");
              cancelModal.open();
            }}
            loading={actionLoading === "cancel"}
          >
            {t("action_cancel")}
          </Button>
        )}
      </Group>

      <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="md">
        <Card withBorder radius="md" padding="lg">
          <Title order={3} size="h5" mb="sm">
            {t("status")}
          </Title>
          <Group gap="xs">
            <StatusBadge value={booking.status} />
            <StatusBadge value={booking.payment_status} />
          </Group>
          <Divider my="sm" />
          <DefList
            rows={[
              [t("source"), booking.source],
              [t("check_in"), booking.check_in_date],
              [t("check_out"), booking.check_out_date],
              [t("nights"), String(booking.nights)],
              ["Rooms", String(booking.room_count)],
            ]}
          />
        </Card>

        <Card withBorder radius="md" padding="lg">
          <Title order={3} size="h5" mb="sm">
            Guest
          </Title>
          <DefList
            rows={[
              [t("guest_name"), booking.guest_name],
              [t("guest_email"), booking.guest_email],
              [t("guest_phone"), booking.guest_phone || "—"],
              [t("country"), booking.guest_country || "—"],
            ]}
          />
          {booking.special_request && (
            <>
              <Divider my="sm" />
              <Text size="xs" tt="uppercase" c="dimmed" mb={4}>
                Special request
              </Text>
              <Text size="sm">{booking.special_request}</Text>
            </>
          )}
        </Card>

        <Card withBorder radius="md" padding="lg" style={{ gridColumn: "1 / -1" }}>
          <Title order={3} size="h5" mb="sm">
            Money
          </Title>
          <SimpleGrid cols={{ base: 2, sm: 4 }} spacing="sm">
            <Money label="Room" value={booking.room_subtotal_cents} currency={booking.currency} />
            <Money label="Taxes" value={booking.taxes_cents} currency={booking.currency} />
            <Money label="Fees" value={booking.fees_cents} currency={booking.currency} />
            <Money label="Discounts" value={-booking.discounts_cents} currency={booking.currency} />
          </SimpleGrid>
          <Divider my="sm" />
          <Group justify="space-between">
            <Text fw={500}>{t("total")}</Text>
            <Text fw={600} ff="monospace">
              {centsToDisplay(booking.total_cents, booking.currency)}
            </Text>
          </Group>
        </Card>

        <Card withBorder radius="md" padding="lg" style={{ gridColumn: "1 / -1" }}>
          <Title order={3} size="h5" mb="md">
            {t("booking_timeline")}
          </Title>
          {events.length > 0 ? (
            <Timeline bulletSize={20} lineWidth={2} active={events.length}>
              {events.map((e) => {
                const labelKey = eventLabelKey[e.event_type];
                const label = labelKey ? t(labelKey as keyof Dict) : e.event_type;
                return (
                  <Timeline.Item key={e.id} title={label}>
                    <Text size="xs" c="dimmed">
                      {formatDateTime(e.created_at, activeHotel.timezone)}
                      {" · "}
                      {t(("actor_" + e.actor_type) as keyof Dict)}
                    </Text>
                  </Timeline.Item>
                );
              })}
            </Timeline>
          ) : (
            <Stack gap="xs">
              {fallbackTimeline.map((e) => (
                <Group key={e.label} gap="md" align="baseline">
                  <Text size="sm" c="dimmed" tt="capitalize" w={120}>
                    {e.label}
                  </Text>
                  <Text size="sm">{formatDateTime(e.at, activeHotel.timezone)}</Text>
                </Group>
              ))}
            </Stack>
          )}
          {booking.cancellation_reason && (
            <Text size="sm" c="dimmed" mt="sm">
              Reason: {booking.cancellation_reason}
            </Text>
          )}
        </Card>
      </SimpleGrid>

      <Modal
        opened={cancelOpened}
        onClose={cancelModal.close}
        title={t("action_cancel")}
        centered
        radius="md"
      >
        <Stack gap="md">
          <Textarea
            label="Reason"
            placeholder="(optional)"
            value={cancelReason}
            onChange={(e) => setCancelReason(e.currentTarget.value)}
            rows={3}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={cancelModal.close}>
              {t("back")}
            </Button>
            <Button
              color="red"
              loading={actionLoading === "cancel"}
              onClick={async () => {
                cancelModal.close();
                await runAction("cancel", cancelReason);
              }}
            >
              {t("action_cancel")}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </div>
  );
}

function DefList({ rows }: { rows: Array<[string, string]> }) {
  return (
    <Stack gap={6}>
      {rows.map(([k, v]) => (
        <Group key={k} justify="space-between" align="baseline">
          <Text size="sm" c="dimmed">
            {k}
          </Text>
          <Text size="sm" ta="right" style={{ wordBreak: "break-word" }}>
            {v}
          </Text>
        </Group>
      ))}
    </Stack>
  );
}

function Money({
  label,
  value,
  currency,
}: {
  label: string;
  value: number;
  currency: string;
}) {
  const negative = value < 0;
  return (
    <Stack gap={2}>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text size="sm" ff="monospace">
        {negative ? "−" : ""}
        {centsToDisplay(Math.abs(value), currency)}
      </Text>
    </Stack>
  );
}
