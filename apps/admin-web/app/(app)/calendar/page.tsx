"use client";

import { useEffect, useMemo, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  EmptyState,
  ErrorBanner,
  Field,
  Modal,
  PageHeader,
  Select,
  TextInput,
} from "@/app/components/ui";
import { Bookings, RoomTypes } from "@/app/lib/api";
import { addDaysISO, daysBetween, monthRange, todayISO } from "@/app/lib/dates";
import { rateToCents } from "@/app/lib/money";
import { t } from "@/app/i18n";
import type { Booking, RoomType } from "@/app/lib/types";

type Cell = { available: number; used: number };

export default function CalendarPage() {
  const { activeHotel } = useShell();

  const [now, setNow] = useState(() => {
    const today = todayISO(activeHotel.timezone);
    const [y, m] = today.split("-").map((n) => parseInt(n, 10));
    return { year: y, monthIndex: m - 1 };
  });

  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [bookings, setBookings] = useState<Booking[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [modalCell, setModalCell] = useState<{ rt: RoomType; date: string } | null>(null);

  const { days } = useMemo(
    () => monthRange(now.year, now.monthIndex),
    [now.year, now.monthIndex]
  );

  useEffect(() => {
    setLoading(true);
    setError(null);
    (async () => {
      try {
        const [rt, b] = await Promise.all([
          RoomTypes.list(activeHotel.id),
          Bookings.list(activeHotel.id, { limit: 500 }),
        ]);
        setRoomTypes(rt.room_types);
        setBookings(b.bookings);
      } catch (e) {
        setError(e instanceof Error ? e.message : t("error_generic"));
      } finally {
        setLoading(false);
      }
    })();
  }, [activeHotel.id]);

  const cells = useMemo(() => {
    const map = new Map<string, Cell>();
    for (const rt of roomTypes) {
      for (const d of days) {
        map.set(`${rt.id}|${d}`, { available: rt.total_inventory, used: 0 });
      }
    }
    for (const b of bookings) {
      if (
        b.status === "cancelled" ||
        b.status === "expired" ||
        b.status === "no_show"
      ) {
        continue;
      }
      const span = daysBetween(b.check_in_date, b.check_out_date);
      for (let i = 0; i < span; i++) {
        const d = addDaysISO(b.check_in_date, i);
        const key = `${b.room_type_id}|${d}`;
        const c = map.get(key);
        if (c) {
          c.used += b.room_count;
          c.available -= b.room_count;
        }
      }
    }
    return map;
  }, [days, roomTypes, bookings]);

  function shiftMonth(delta: number) {
    setNow(({ year, monthIndex }) => {
      const next = new Date(Date.UTC(year, monthIndex + delta, 1));
      return { year: next.getUTCFullYear(), monthIndex: next.getUTCMonth() };
    });
  }

  return (
    <div>
      <PageHeader
        title={t("calendar_title")}
        description={activeHotel.name}
        actions={
          <>
            <Button variant="secondary" onClick={() => shiftMonth(-1)}>
              ← {t("prev_month")}
            </Button>
            <Button variant="secondary" onClick={() => shiftMonth(1)}>
              {t("next_month")} →
            </Button>
          </>
        }
      />

      <p className="mb-3 text-sm text-neutral-600">
        {new Date(Date.UTC(now.year, now.monthIndex, 1)).toLocaleDateString("en-GB", {
          month: "long",
          year: "numeric",
        })}
      </p>

      <ErrorBanner message={error} />

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : roomTypes.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-neutral-200 bg-white">
          <table className="min-w-full border-collapse text-xs">
            <thead>
              <tr className="bg-neutral-50">
                <th className="sticky left-0 z-10 min-w-[160px] border-b border-r border-neutral-200 bg-neutral-50 px-3 py-2 text-left font-medium text-neutral-600">
                  {t("nav_room_types")}
                </th>
                {days.map((d) => {
                  const dayNum = d.split("-")[2];
                  return (
                    <th
                      key={d}
                      className="border-b border-neutral-200 px-1 py-2 text-center font-medium text-neutral-500"
                    >
                      {dayNum}
                    </th>
                  );
                })}
              </tr>
            </thead>
            <tbody>
              {roomTypes.map((rt) => (
                <tr key={rt.id}>
                  <td className="sticky left-0 z-10 min-w-[160px] border-b border-r border-neutral-200 bg-white px-3 py-2 font-medium text-neutral-700">
                    {rt.name}
                    <span className="ml-1 text-neutral-400">
                      ({rt.total_inventory})
                    </span>
                  </td>
                  {days.map((d) => {
                    const c = cells.get(`${rt.id}|${d}`) || {
                      available: rt.total_inventory,
                      used: 0,
                    };
                    const ratio = rt.total_inventory
                      ? c.used / rt.total_inventory
                      : 0;
                    const tone =
                      ratio === 0
                        ? "bg-white"
                        : ratio < 0.5
                          ? "bg-emerald-50"
                          : ratio < 0.9
                            ? "bg-amber-100"
                            : "bg-red-200";
                    return (
                      <td
                        key={d}
                        onClick={() => setModalCell({ rt, date: d })}
                        className={`min-w-[36px] cursor-pointer border-b border-l border-neutral-100 px-1 py-2 text-center ${tone} hover:ring-1 hover:ring-neutral-400`}
                        title={`${rt.name} on ${d} — ${c.used}/${rt.total_inventory}`}
                      >
                        {c.used > 0 ? c.used : ""}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <WalkInModal
        open={!!modalCell}
        onClose={() => setModalCell(null)}
        hotelID={activeHotel.id}
        baseCurrency={activeHotel.base_currency}
        cell={modalCell}
        onCreated={async () => {
          setModalCell(null);
          const b = await Bookings.list(activeHotel.id, { limit: 500 });
          setBookings(b.bookings);
        }}
      />
    </div>
  );
}

function WalkInModal({
  open,
  onClose,
  hotelID,
  baseCurrency,
  cell,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  hotelID: string;
  baseCurrency: string;
  cell: { rt: RoomType; date: string } | null;
  onCreated: () => Promise<void>;
}) {
  const [guestName, setGuestName] = useState("");
  const [guestEmail, setGuestEmail] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [nights, setNights] = useState(1);
  const [rooms, setRooms] = useState(1);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setGuestName("");
      setGuestEmail("");
      setGuestPhone("");
      setNights(1);
      setRooms(1);
      setErr(null);
    }
  }, [open, cell?.date, cell?.rt.id]);

  if (!cell) return null;

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!cell) return;
    setLoading(true);
    setErr(null);
    try {
      const checkOut = addDaysISO(cell.date, nights);
      const subtotal = rateToCents(cell.rt.base_rate) * nights * rooms;
      await Bookings.create(hotelID, {
        room_type_id: cell.rt.id,
        room_count: rooms,
        check_in_date: cell.date,
        check_out_date: checkOut,
        guest_email: guestEmail,
        guest_name: guestName,
        guest_phone: guestPhone || undefined,
        currency: cell.rt.base_currency || baseCurrency,
        room_subtotal_cents: subtotal,
        total_cents: subtotal,
      });
      await onCreated();
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={t("walk_in_booking")}>
      <form onSubmit={submit} className="space-y-3">
        <p className="text-xs text-neutral-500">
          {cell.rt.name} • {cell.date}
        </p>
        <Field label={t("guest_name")}>
          <TextInput required value={guestName} onChange={(e) => setGuestName(e.target.value)} />
        </Field>
        <Field label={t("guest_email")}>
          <TextInput
            type="email"
            required
            value={guestEmail}
            onChange={(e) => setGuestEmail(e.target.value)}
          />
        </Field>
        <Field label={t("guest_phone")}>
          <TextInput value={guestPhone} onChange={(e) => setGuestPhone(e.target.value)} />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("nights")}>
            <TextInput
              type="number"
              min={1}
              required
              value={nights}
              onChange={(e) => setNights(Math.max(1, parseInt(e.target.value || "1", 10)))}
            />
          </Field>
          <Field label="Rooms">
            <Select
              value={rooms}
              onChange={(e) => setRooms(parseInt(e.target.value, 10))}
            >
              {Array.from({ length: cell.rt.total_inventory || 1 }, (_, i) => i + 1).map(
                (n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                )
              )}
            </Select>
          </Field>
        </div>
        <ErrorBanner message={err} />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" loading={loading}>
            {t("create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
