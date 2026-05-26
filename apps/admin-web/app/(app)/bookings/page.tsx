"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  EmptyState,
  ErrorBanner,
  Field,
  PageHeader,
  Select,
  StatusBadge,
  TextInput,
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

      <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-4">
        <Field label={t("filter_status")}>
          <Select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">{t("all")}</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s.replace(/_/g, " ")}
              </option>
            ))}
          </Select>
        </Field>
        <Field label={t("check_in") + " ≥"}>
          <TextInput type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
        </Field>
        <Field label={t("check_out") + " ≤"}>
          <TextInput type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        </Field>
      </div>

      <ErrorBanner message={error} />

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : filtered.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-neutral-200 bg-white">
          <table className="w-full min-w-[800px] text-sm">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500">
                <th className="px-4 py-2.5">{t("booking_reference")}</th>
                <th>{t("guest_name")}</th>
                <th>{t("check_in")}</th>
                <th>{t("check_out")}</th>
                <th>{t("status")}</th>
                <th>{t("source")}</th>
                <th className="px-4 text-right">{t("total")}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((b) => (
                <tr key={b.id} className="border-b border-neutral-100 last:border-0 hover:bg-neutral-50">
                  <td className="px-4 py-2.5">
                    <Link
                      href={`/bookings/${b.id}`}
                      className="font-mono text-xs text-neutral-900 underline"
                    >
                      {b.reference}
                    </Link>
                  </td>
                  <td>{b.guest_name}</td>
                  <td>{b.check_in_date}</td>
                  <td>{b.check_out_date}</td>
                  <td>
                    <StatusBadge value={b.status} />
                  </td>
                  <td className="text-xs text-neutral-600">{b.source}</td>
                  <td className="px-4 text-right font-mono">
                    {centsToDisplay(b.total_cents, b.currency)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
