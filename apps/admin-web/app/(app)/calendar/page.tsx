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
  SuccessBanner,
  TextInput,
} from "@/app/components/ui";
import { Availability, Bookings, RoomTypes } from "@/app/lib/api";
import { addDaysISO, daysBetween, monthRange, todayISO } from "@/app/lib/dates";
import { parseDisplayToCents, rateToCents } from "@/app/lib/money";
import { t } from "@/app/i18n";
import type {
  Booking,
  RoomType,
  UpsertAvailabilityItem,
} from "@/app/lib/types";

type Cell = { available: number; used: number };
type SelKey = string; // `${rt.id}|${date}`

function cellKey(rtID: string, date: string): SelKey {
  return `${rtID}|${date}`;
}

// Formats user input (e.g. "1500" or "1,500.50") into the decimal string the
// backend expects for rate_override (NUMERIC(10,2)). Returns null if invalid.
// We round-trip through satang to enforce two-decimal precision.
function toRateOverride(input: string): string | null {
  const c = parseDisplayToCents(input);
  if (!Number.isFinite(c) || Number.isNaN(c)) return null;
  const sign = c < 0 ? "-" : "";
  const abs = Math.abs(c);
  return `${sign}${Math.floor(abs / 100)}.${(abs % 100).toString().padStart(2, "0")}`;
}

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

  // Bulk-edit mode state. When bulkMode is true, cells toggle selection
  // instead of opening the walk-in modal. lastSel anchors shift-range.
  const [bulkMode, setBulkMode] = useState(false);
  const [selected, setSelected] = useState<Set<SelKey>>(new Set());
  const [lastSel, setLastSel] = useState<{ rtID: string; date: string } | null>(null);

  const [bulkInv, setBulkInv] = useState("");
  const [bulkRate, setBulkRate] = useState("");
  const [bulkBlocked, setBulkBlocked] = useState(false);
  const [bulkSaving, setBulkSaving] = useState(false);
  const [bulkErr, setBulkErr] = useState<string | null>(null);
  const [bulkSuccess, setBulkSuccess] = useState<string | null>(null);

  const { days } = useMemo(
    () => monthRange(now.year, now.monthIndex),
    [now.year, now.monthIndex]
  );

  async function reload() {
    const [rt, b] = await Promise.all([
      RoomTypes.list(activeHotel.id),
      Bookings.list(activeHotel.id, { limit: 500 }),
    ]);
    setRoomTypes(rt.room_types);
    setBookings(b.bookings);
  }

  useEffect(() => {
    setLoading(true);
    setError(null);
    (async () => {
      try {
        await reload();
      } catch (e) {
        setError(e instanceof Error ? e.message : t("error_generic"));
      } finally {
        setLoading(false);
      }
    })();
  }, [activeHotel.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const cells = useMemo(() => {
    const map = new Map<string, Cell>();
    for (const rt of roomTypes) {
      for (const d of days) {
        map.set(cellKey(rt.id, d), { available: rt.total_inventory, used: 0 });
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
        const key = cellKey(b.room_type_id, d);
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

  function clearSelection() {
    setSelected(new Set());
    setLastSel(null);
  }

  function toggleBulkMode() {
    setBulkMode((m) => !m);
    clearSelection();
    setBulkInv("");
    setBulkRate("");
    setBulkBlocked(false);
    setBulkErr(null);
    setBulkSuccess(null);
  }

  function onCellClick(rt: RoomType, date: string, e: React.MouseEvent) {
    if (!bulkMode) {
      setModalCell({ rt, date });
      return;
    }
    const next = new Set(selected);
    const k = cellKey(rt.id, date);
    // Shift-range only within the same room-type row, since the override grid
    // is per (room_type_id, date) and a cross-row range rarely matches intent.
    if (e.shiftKey && lastSel && lastSel.rtID === rt.id) {
      const i0 = days.indexOf(lastSel.date);
      const i1 = days.indexOf(date);
      if (i0 >= 0 && i1 >= 0) {
        const [a, b] = i0 < i1 ? [i0, i1] : [i1, i0];
        for (let i = a; i <= b; i++) next.add(cellKey(rt.id, days[i]));
        setSelected(next);
        setLastSel({ rtID: rt.id, date });
        return;
      }
    }
    if (next.has(k)) next.delete(k);
    else next.add(k);
    setSelected(next);
    setLastSel({ rtID: rt.id, date });
  }

  function buildItem(rt: RoomType, date: string): UpsertAvailabilityItem | null {
    const item: UpsertAvailabilityItem = {
      room_type_id: rt.id,
      date,
    };
    let touched = false;
    if (bulkInv.trim() !== "") {
      const target = parseInt(bulkInv, 10);
      if (!Number.isNaN(target)) {
        item.inventory_change = target - rt.total_inventory;
        touched = true;
      }
    }
    if (bulkRate.trim() !== "") {
      const rateStr = toRateOverride(bulkRate);
      if (rateStr === null) return null;
      item.rate_override = rateStr;
      item.rate_currency = rt.base_currency || activeHotel.base_currency;
      touched = true;
    }
    if (bulkBlocked) {
      item.closed = true;
      touched = true;
    }
    return touched ? item : null;
  }

  async function saveOverrides() {
    setBulkErr(null);
    setBulkSuccess(null);
    if (bulkRate.trim() !== "" && toRateOverride(bulkRate) === null) {
      setBulkErr(t("bulk_invalid_rate"));
      return;
    }
    const items: UpsertAvailabilityItem[] = [];
    for (const k of selected) {
      const [rtID, date] = k.split("|");
      const rt = roomTypes.find((r) => r.id === rtID);
      if (!rt) continue;
      const item = buildItem(rt, date);
      if (item) items.push(item);
    }
    if (items.length === 0) {
      setBulkErr(t("no_data"));
      return;
    }
    setBulkSaving(true);
    try {
      await Availability.upsert(activeHotel.id, items);
      clearSelection();
      setBulkSuccess(t("bulk_saved"));
      await reload();
    } catch (e) {
      setBulkErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setBulkSaving(false);
    }
  }

  async function clearOverrides() {
    setBulkErr(null);
    setBulkSuccess(null);
    if (selected.size === 0) return;
    setBulkSaving(true);
    try {
      for (const k of selected) {
        const [rtID, date] = k.split("|");
        await Availability.remove(activeHotel.id, rtID, date);
      }
      clearSelection();
      setBulkSuccess(t("bulk_cleared"));
      await reload();
    } catch (e) {
      setBulkErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setBulkSaving(false);
    }
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
            <Button
              variant={bulkMode ? "primary" : "secondary"}
              onClick={toggleBulkMode}
            >
              {bulkMode ? t("exit_bulk_edit") : t("bulk_edit")}
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
      {bulkMode && (
        <p className="mb-3 text-xs text-neutral-500">{t("bulk_hint")}</p>
      )}

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : roomTypes.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div className={`overflow-x-auto rounded-lg border border-neutral-200 bg-white ${bulkMode ? "pb-40" : ""}`}>
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
                    const c = cells.get(cellKey(rt.id, d)) || {
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
                    const isSel = selected.has(cellKey(rt.id, d));
                    const selRing = isSel ? "ring-2 ring-blue-500 ring-inset" : "";
                    return (
                      <td
                        key={d}
                        onClick={(e) => onCellClick(rt, d, e)}
                        className={`min-w-[36px] cursor-pointer border-b border-l border-neutral-100 px-1 py-2 text-center ${tone} ${selRing} hover:ring-1 hover:ring-neutral-400`}
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

      <SuccessBanner message={bulkSuccess} />

      {bulkMode && selected.size > 0 && (
        <BulkEditPanel
          count={selected.size}
          inv={bulkInv}
          rate={bulkRate}
          blocked={bulkBlocked}
          baseCurrency={activeHotel.base_currency}
          saving={bulkSaving}
          err={bulkErr}
          onInvChange={setBulkInv}
          onRateChange={setBulkRate}
          onBlockedChange={setBulkBlocked}
          onClearSelection={clearSelection}
          onClear={clearOverrides}
          onSave={saveOverrides}
        />
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

function BulkEditPanel({
  count,
  inv,
  rate,
  blocked,
  baseCurrency,
  saving,
  err,
  onInvChange,
  onRateChange,
  onBlockedChange,
  onClearSelection,
  onClear,
  onSave,
}: {
  count: number;
  inv: string;
  rate: string;
  blocked: boolean;
  baseCurrency: string;
  saving: boolean;
  err: string | null;
  onInvChange: (v: string) => void;
  onRateChange: (v: string) => void;
  onBlockedChange: (v: boolean) => void;
  onClearSelection: () => void;
  onClear: () => void;
  onSave: () => void;
}) {
  return (
    <div className="fixed inset-x-0 bottom-0 z-30 border-t border-neutral-200 bg-white px-4 py-3 shadow-lg">
      <div className="mx-auto flex max-w-6xl flex-wrap items-end gap-3">
        <div className="text-sm font-medium text-neutral-700">
          {count} {t("bulk_selected")}
        </div>
        <Field label={t("bulk_set_inventory")}>
          <TextInput
            type="number"
            min={0}
            value={inv}
            onChange={(e) => onInvChange(e.target.value)}
            className="w-28"
          />
        </Field>
        <Field label={`${t("bulk_set_rate")} (${baseCurrency})`}>
          <TextInput
            value={rate}
            onChange={(e) => onRateChange(e.target.value)}
            placeholder="1500.00"
            className="w-32"
          />
        </Field>
        <label className="inline-flex items-center gap-1 text-sm">
          <input
            type="checkbox"
            checked={blocked}
            onChange={(e) => onBlockedChange(e.target.checked)}
          />
          {t("bulk_block_sales")}
        </label>
        <div className="ml-auto flex gap-2">
          <Button variant="ghost" onClick={onClearSelection} disabled={saving}>
            {t("bulk_clear_selection")}
          </Button>
          <Button variant="secondary" onClick={onClear} loading={saving}>
            {t("bulk_clear_overrides")}
          </Button>
          <Button onClick={onSave} loading={saving}>
            {t("bulk_save_overrides")}
          </Button>
        </div>
      </div>
      {err && (
        <div className="mx-auto mt-2 max-w-6xl">
          <ErrorBanner message={err} />
        </div>
      )}
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
