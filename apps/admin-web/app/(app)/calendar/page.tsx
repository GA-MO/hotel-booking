"use client";

import { useEffect, useMemo, useState } from "react";

import {
  Button,
  Center,
  Checkbox,
  Group,
  Loader,
  Modal,
  NativeSelect,
  NumberInput,
  Paper,
  Stack,
  Text,
  TextInput,
} from "@mantine/core";
import { useForm } from "@mantine/form";

import { useShell } from "@/app/components/AppShell";
import {
  EmptyState,
  ErrorBanner,
  PageHeader,
} from "@/app/components/ui";
import { Availability, Bookings, RoomTypes } from "@/app/lib/api";
import { addDaysISO, daysBetween, monthRange, todayISO } from "@/app/lib/dates";
import { parseDisplayToCents, rateToCents } from "@/app/lib/money";
import { notifySuccess } from "@/app/lib/notify";
import { t } from "@/app/i18n";
import type {
  AvailabilityDay,
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

// hasOverride compares an AvailabilityDay (server-resolved) against the
// RoomType defaults to decide whether an `availability_overrides` row likely
// exists for it. Used as a UX hint only — the authoritative state still lives
// in the DB.
function hasOverride(rt: RoomType, day: AvailabilityDay): boolean {
  if (day.closed) return true;
  if (day.total_inventory !== rt.total_inventory) return true;
  const dayRate = parseFloat(day.rate);
  return Number.isFinite(dayRate) && Math.abs(dayRate - rt.base_rate) > 0.005;
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
  const [overrides, setOverrides] = useState<Map<SelKey, AvailabilityDay>>(
    new Map()
  );
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [modalCell, setModalCell] = useState<{ rt: RoomType; date: string } | null>(null);

  const [bulkMode, setBulkMode] = useState(false);
  const [selected, setSelected] = useState<Set<SelKey>>(new Set());
  const [lastSel, setLastSel] = useState<{ rtID: string; date: string } | null>(null);

  const [bulkInv, setBulkInv] = useState("");
  const [bulkRate, setBulkRate] = useState("");
  const [bulkBlocked, setBulkBlocked] = useState(false);
  const [bulkSaving, setBulkSaving] = useState(false);
  const [bulkErr, setBulkErr] = useState<string | null>(null);

  const { days } = useMemo(
    () => monthRange(now.year, now.monthIndex),
    [now.year, now.monthIndex]
  );

  async function reload() {
    const start = days[0];
    const end = addDaysISO(days[days.length - 1], 1); // exclusive
    const [rt, b, av] = await Promise.all([
      RoomTypes.list(activeHotel.id),
      Bookings.list(activeHotel.id, { limit: 500 }),
      Availability.get(activeHotel.id, start, end).catch(
        () => ({ days: [] } as { days: AvailabilityDay[] }),
      ),
    ]);
    setRoomTypes(rt.room_types);
    setBookings(b.bookings);
    const m = new Map<SelKey, AvailabilityDay>();
    for (const d of av.days) m.set(cellKey(d.room_type_id, d.date), d);
    setOverrides(m);
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
  }, [activeHotel.id, now.year, now.monthIndex]); // eslint-disable-line react-hooks/exhaustive-deps

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

  useEffect(() => {
    if (!bulkMode || selected.size !== 1) return;
    const [only] = Array.from(selected);
    const day = overrides.get(only);
    const [rtID] = only.split("|");
    const rt = roomTypes.find((r) => r.id === rtID);
    if (!day || !rt || !hasOverride(rt, day)) return;
    setBulkInv(
      day.total_inventory !== rt.total_inventory ? String(day.total_inventory) : ""
    );
    setBulkRate(
      Math.abs(parseFloat(day.rate) - rt.base_rate) > 0.005 ? day.rate : "",
    );
    setBulkBlocked(day.closed);
  }, [bulkMode, selected, overrides, roomTypes]);

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
  }

  function onCellClick(rt: RoomType, date: string, e: React.MouseEvent) {
    if (!bulkMode) {
      setModalCell({ rt, date });
      return;
    }
    const next = new Set(selected);
    const k = cellKey(rt.id, date);
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
      notifySuccess(t("bulk_saved"));
      await reload();
    } catch (e) {
      setBulkErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setBulkSaving(false);
    }
  }

  async function clearOverrides() {
    setBulkErr(null);
    if (selected.size === 0) return;
    setBulkSaving(true);
    try {
      for (const k of selected) {
        const [rtID, date] = k.split("|");
        await Availability.remove(activeHotel.id, rtID, date);
      }
      clearSelection();
      notifySuccess(t("bulk_cleared"));
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
          <Group gap="xs">
            <Button variant="default" size="sm" onClick={() => shiftMonth(-1)}>
              ← {t("prev_month")}
            </Button>
            <Button variant="default" size="sm" onClick={() => shiftMonth(1)}>
              {t("next_month")} →
            </Button>
            <Button
              variant={bulkMode ? "filled" : "default"}
              color={bulkMode ? "dark" : undefined}
              size="sm"
              onClick={toggleBulkMode}
            >
              {bulkMode ? t("exit_bulk_edit") : t("bulk_edit")}
            </Button>
          </Group>
        }
      />

      <Text size="sm" c="dimmed" mb="sm">
        {new Date(Date.UTC(now.year, now.monthIndex, 1)).toLocaleDateString("en-GB", {
          month: "long",
          year: "numeric",
        })}
      </Text>

      <ErrorBanner message={error} />
      {bulkMode && (
        <Text size="xs" c="dimmed" mb="sm">
          {t("bulk_hint")}
        </Text>
      )}

      {loading ? (
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      ) : roomTypes.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div
          className={`overflow-x-auto rounded-lg border border-neutral-200 bg-white ${bulkMode ? "pb-40" : ""}`}
        >
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
                    const day = overrides.get(cellKey(rt.id, d));
                    const overridden = day ? hasOverride(rt, day) : false;
                    const title = overridden && day
                      ? `${rt.name} on ${d} — ${c.used}/${day.total_inventory}` +
                        (day.closed ? " · BLOCKED" : "") +
                        (Math.abs(parseFloat(day.rate) - rt.base_rate) > 0.005
                          ? ` · rate ${day.rate}`
                          : "")
                      : `${rt.name} on ${d} — ${c.used}/${rt.total_inventory}`;
                    return (
                      <td
                        key={d}
                        onClick={(e) => onCellClick(rt, d, e)}
                        className={`relative min-w-[36px] cursor-pointer border-b border-l border-neutral-100 px-1 py-2 text-center ${tone} ${selRing} hover:ring-1 hover:ring-neutral-400`}
                        title={title}
                      >
                        {c.used > 0 ? c.used : ""}
                        {overridden && (
                          <span
                            aria-hidden
                            className={`absolute right-0.5 top-0.5 inline-block h-1.5 w-1.5 rounded-full ${day?.closed ? "bg-red-500" : "bg-indigo-500"}`}
                          />
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

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
        cell={modalCell}
        onClose={() => setModalCell(null)}
        hotelID={activeHotel.id}
        baseCurrency={activeHotel.base_currency}
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
    <Paper
      withBorder
      shadow="lg"
      pos="fixed"
      bottom={0}
      left={0}
      right={0}
      style={{ zIndex: 30 }}
      p="md"
      bg="white"
    >
      <Group justify="space-between" align="end" wrap="wrap" gap="md" maw={1200} mx="auto">
        <Group gap="md" align="end" wrap="wrap">
          <Text size="sm" fw={500}>
            {count} {t("bulk_selected")}
          </Text>
          <TextInput
            label={t("bulk_set_inventory")}
            type="number"
            min={0}
            value={inv}
            onChange={(e) => onInvChange(e.currentTarget.value)}
            w={120}
          />
          <TextInput
            label={`${t("bulk_set_rate")} (${baseCurrency})`}
            value={rate}
            onChange={(e) => onRateChange(e.currentTarget.value)}
            placeholder="1500.00"
            w={140}
          />
          <Checkbox
            label={t("bulk_block_sales")}
            checked={blocked}
            onChange={(e) => onBlockedChange(e.currentTarget.checked)}
            mb={6}
          />
        </Group>
        <Group gap="xs">
          <Button
            variant="subtle"
            color="gray"
            onClick={onClearSelection}
            disabled={saving}
          >
            {t("bulk_clear_selection")}
          </Button>
          <Button variant="default" onClick={onClear} loading={saving}>
            {t("bulk_clear_overrides")}
          </Button>
          <Button color="dark" onClick={onSave} loading={saving}>
            {t("bulk_save_overrides")}
          </Button>
        </Group>
      </Group>
      {err && (
        <div style={{ maxWidth: 1200, margin: "8px auto 0" }}>
          <ErrorBanner message={err} />
        </div>
      )}
    </Paper>
  );
}

function WalkInModal({
  cell,
  onClose,
  hotelID,
  baseCurrency,
  onCreated,
}: {
  cell: { rt: RoomType; date: string } | null;
  onClose: () => void;
  hotelID: string;
  baseCurrency: string;
  onCreated: () => Promise<void>;
}) {
  const opened = !!cell;
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const form = useForm({
    initialValues: {
      guestName: "",
      guestEmail: "",
      guestPhone: "",
      nights: 1,
      rooms: 1,
    },
    validate: {
      guestName: (v) => (v.trim() ? null : t("required_field")),
      guestEmail: (v) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? null : t("invalid_email"),
    },
  });

  useEffect(() => {
    if (opened) {
      form.reset();
      setErr(null);
    }
    // form.reset only depends on form itself; safe to omit from deps
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opened, cell?.date, cell?.rt.id]);

  if (!cell) {
    return (
      <Modal opened={false} onClose={onClose} title={t("walk_in_booking")}>
        <div />
      </Modal>
    );
  }

  async function submit(values: typeof form.values) {
    if (!cell) return;
    setLoading(true);
    setErr(null);
    try {
      const checkOut = addDaysISO(cell.date, values.nights);
      const subtotal =
        rateToCents(cell.rt.base_rate) * values.nights * values.rooms;
      await Bookings.create(hotelID, {
        room_type_id: cell.rt.id,
        room_count: values.rooms,
        check_in_date: cell.date,
        check_out_date: checkOut,
        guest_email: values.guestEmail,
        guest_name: values.guestName,
        guest_phone: values.guestPhone || undefined,
        currency: cell.rt.base_currency || baseCurrency,
        room_subtotal_cents: subtotal,
        total_cents: subtotal,
      });
      notifySuccess(`${t("walk_in_booking")} ✓`);
      await onCreated();
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setLoading(false);
    }
  }

  const roomOptions = Array.from(
    { length: cell.rt.total_inventory || 1 },
    (_, i) => String(i + 1)
  );

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={t("walk_in_booking")}
      centered
      radius="md"
    >
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap="sm">
          <Text size="xs" c="dimmed">
            {cell.rt.name} • {cell.date}
          </Text>
          <TextInput
            label={t("guest_name")}
            required
            {...form.getInputProps("guestName")}
          />
          <TextInput
            label={t("guest_email")}
            type="email"
            required
            {...form.getInputProps("guestEmail")}
          />
          <TextInput
            label={t("guest_phone")}
            {...form.getInputProps("guestPhone")}
          />
          <Group grow>
            <NumberInput
              label={t("nights")}
              min={1}
              required
              {...form.getInputProps("nights")}
            />
            <NativeSelect
              label="Rooms"
              data={roomOptions}
              value={String(form.values.rooms)}
              onChange={(e) =>
                form.setFieldValue("rooms", parseInt(e.currentTarget.value, 10))
              }
            />
          </Group>
          <ErrorBanner message={err} />
          <Group justify="flex-end" mt="sm">
            <Button type="button" variant="default" onClick={onClose}>
              {t("cancel")}
            </Button>
            <Button type="submit" loading={loading} color="dark">
              {t("create")}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
