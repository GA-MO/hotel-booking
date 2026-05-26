"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  Card,
  ErrorBanner,
  PageHeader,
  StatusBadge,
} from "@/app/components/ui";
import { Bookings, ApiClientError } from "@/app/lib/api";
import { formatDateTime } from "@/app/lib/dates";
import { centsToDisplay } from "@/app/lib/money";
import { t } from "@/app/i18n";
import type { Booking } from "@/app/lib/types";

type ActionKey = "confirm" | "checkIn" | "checkOut" | "cancel" | "noShow";

export default function BookingDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { activeHotel } = useShell();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<ActionKey | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const b = await Bookings.get(activeHotel.id, params.id);
      setBooking(b);
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

  async function doAction(key: ActionKey) {
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
        case "cancel": {
          const reason = window.prompt("Reason?") || "";
          updated = await Bookings.cancel(activeHotel.id, booking.id, reason);
          break;
        }
      }
      setBooking(updated);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setActionLoading(null);
    }
  }

  if (loading) {
    return <p className="text-sm text-neutral-500">{t("loading")}</p>;
  }
  if (!booking) {
    return <ErrorBanner message={error || "Not found"} />;
  }

  const canConfirm = booking.status === "pending_payment";
  const canCheckIn = booking.status === "confirmed";
  const canCheckOut = booking.status === "checked_in";
  const canCancel = ["pending_payment", "confirmed"].includes(booking.status);
  const canNoShow = booking.status === "confirmed";

  // Booking events aren't exposed via the API yet; we approximate a timeline
  // from the status timestamps that ARE on the booking record.
  const timeline: Array<{ label: string; at?: string | null }> = [
    { label: "created", at: booking.created_at },
    { label: "confirmed", at: booking.confirmed_at },
    { label: "checked in", at: booking.checked_in_at },
    { label: "checked out", at: booking.checked_out_at },
    { label: "cancelled", at: booking.cancelled_at },
  ].filter((e) => e.at);

  return (
    <div>
      <PageHeader
        title={booking.reference}
        description={`${booking.guest_name} — ${activeHotel.name}`}
        actions={
          <Link
            href="/bookings"
            className="rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm hover:bg-neutral-100"
          >
            ← {t("nav_bookings")}
          </Link>
        }
      />

      <ErrorBanner message={error} />

      <div className="mb-4 flex flex-wrap gap-2">
        {canConfirm && (
          <Button
            onClick={() => doAction("confirm")}
            loading={actionLoading === "confirm"}
          >
            {t("action_confirm")}
          </Button>
        )}
        {canCheckIn && (
          <Button
            onClick={() => doAction("checkIn")}
            loading={actionLoading === "checkIn"}
          >
            {t("action_check_in")}
          </Button>
        )}
        {canCheckOut && (
          <Button
            onClick={() => doAction("checkOut")}
            loading={actionLoading === "checkOut"}
          >
            {t("action_check_out")}
          </Button>
        )}
        {canNoShow && (
          <Button
            variant="secondary"
            onClick={() => doAction("noShow")}
            loading={actionLoading === "noShow"}
          >
            {t("action_no_show")}
          </Button>
        )}
        {canCancel && (
          <Button
            variant="danger"
            onClick={() => doAction("cancel")}
            loading={actionLoading === "cancel"}
          >
            {t("action_cancel")}
          </Button>
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title={t("status")}>
          <div className="flex items-center gap-2">
            <StatusBadge value={booking.status} />
            <StatusBadge value={booking.payment_status} />
          </div>
          <dl className="mt-4 grid grid-cols-2 gap-y-2 text-sm">
            <dt className="text-neutral-500">{t("source")}</dt>
            <dd>{booking.source}</dd>
            <dt className="text-neutral-500">{t("check_in")}</dt>
            <dd>{booking.check_in_date}</dd>
            <dt className="text-neutral-500">{t("check_out")}</dt>
            <dd>{booking.check_out_date}</dd>
            <dt className="text-neutral-500">{t("nights")}</dt>
            <dd>{booking.nights}</dd>
            <dt className="text-neutral-500">Rooms</dt>
            <dd>{booking.room_count}</dd>
          </dl>
        </Card>

        <Card title="Guest">
          <dl className="grid grid-cols-2 gap-y-2 text-sm">
            <dt className="text-neutral-500">{t("guest_name")}</dt>
            <dd>{booking.guest_name}</dd>
            <dt className="text-neutral-500">{t("guest_email")}</dt>
            <dd className="break-all">{booking.guest_email}</dd>
            <dt className="text-neutral-500">{t("guest_phone")}</dt>
            <dd>{booking.guest_phone || "—"}</dd>
            <dt className="text-neutral-500">{t("country")}</dt>
            <dd>{booking.guest_country || "—"}</dd>
          </dl>
          {booking.special_request && (
            <div className="mt-4">
              <p className="text-xs uppercase tracking-wide text-neutral-500">
                Special request
              </p>
              <p className="mt-1 text-sm">{booking.special_request}</p>
            </div>
          )}
        </Card>

        <Card title="Money" className="lg:col-span-2">
          <dl className="grid grid-cols-2 gap-y-2 text-sm sm:grid-cols-4">
            <dt className="text-neutral-500">Room</dt>
            <dd className="font-mono">
              {centsToDisplay(booking.room_subtotal_cents, booking.currency)}
            </dd>
            <dt className="text-neutral-500">Taxes</dt>
            <dd className="font-mono">
              {centsToDisplay(booking.taxes_cents, booking.currency)}
            </dd>
            <dt className="text-neutral-500">Fees</dt>
            <dd className="font-mono">
              {centsToDisplay(booking.fees_cents, booking.currency)}
            </dd>
            <dt className="text-neutral-500">Discounts</dt>
            <dd className="font-mono">
              −{centsToDisplay(booking.discounts_cents, booking.currency)}
            </dd>
            <dt className="font-medium text-neutral-700">{t("total")}</dt>
            <dd className="font-mono font-semibold">
              {centsToDisplay(booking.total_cents, booking.currency)}
            </dd>
          </dl>
        </Card>

        <Card title="Timeline" className="lg:col-span-2">
          <ol className="space-y-2">
            {timeline.map((e) => (
              <li key={e.label} className="flex items-baseline gap-3 text-sm">
                <span className="w-28 capitalize text-neutral-500">
                  {e.label}
                </span>
                <span>{formatDateTime(e.at as string, activeHotel.timezone)}</span>
              </li>
            ))}
          </ol>
          {booking.cancellation_reason && (
            <p className="mt-3 text-sm text-neutral-600">
              Reason: {booking.cancellation_reason}
            </p>
          )}
        </Card>
      </div>
    </div>
  );
}
