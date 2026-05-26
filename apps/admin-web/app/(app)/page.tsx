"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Card,
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
    (b) => b.check_in_date === today && (b.status === "confirmed" || b.status === "checked_in")
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
          <Link
            href="/bookings"
            className="rounded-md border border-neutral-300 bg-white px-3 py-2 text-sm hover:bg-neutral-100"
          >
            {t("nav_bookings")}
          </Link>
        }
      />

      <ErrorBanner message={error} />

      {sub && (
        <div className="mb-6 rounded-md border border-neutral-200 bg-white px-4 py-3 text-sm">
          <span className="mr-2 font-medium text-neutral-700">
            {t("subscription_status")}:
          </span>
          <StatusBadge value={sub.status} />
        </div>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card title={t("today_check_ins")}>
          <p className="text-3xl font-semibold">{todayCheckIns.length}</p>
        </Card>
        <Card title={t("occupancy_today")}>
          <p className="text-3xl font-semibold">{occupancy}%</p>
          <p className="mt-1 text-xs text-neutral-500">
            {occupiedToday} / {totalInventory}
          </p>
        </Card>
        <Card title={t("nav_room_types")}>
          <p className="text-3xl font-semibold">{roomTypes.length}</p>
        </Card>
      </div>

      <div className="mt-6">
        <Card title={t("recent_bookings")}>
          {loading ? (
            <p className="text-sm text-neutral-500">{t("loading")}</p>
          ) : recent.length === 0 ? (
            <EmptyState message={t("no_data")} />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-neutral-200 text-left text-xs uppercase tracking-wide text-neutral-500">
                    <th className="py-2">{t("booking_reference")}</th>
                    <th>{t("guest_name")}</th>
                    <th>{t("check_in")}</th>
                    <th>{t("status")}</th>
                    <th className="text-right">{t("total")}</th>
                  </tr>
                </thead>
                <tbody>
                  {recent.map((b) => (
                    <tr key={b.id} className="border-b border-neutral-100">
                      <td className="py-2">
                        <Link
                          href={`/bookings/${b.id}`}
                          className="font-mono text-xs text-neutral-700 underline"
                        >
                          {b.reference}
                        </Link>
                      </td>
                      <td>{b.guest_name}</td>
                      <td>{b.check_in_date}</td>
                      <td>
                        <StatusBadge value={b.status} />
                      </td>
                      <td className="text-right font-mono">
                        {centsToDisplay(b.total_cents, b.currency)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}
