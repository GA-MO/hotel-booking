"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Field,
  Modal,
  PageHeader,
  Select,
  TextInput,
} from "@/app/components/ui";
import { Pricing, RoomTypes } from "@/app/lib/api";
import { t } from "@/app/i18n";
import type {
  CreatePricingRuleRequest,
  PricingModifierType,
  PricingRule,
  PricingRuleType,
  RoomType,
} from "@/app/lib/types";

const RULE_TYPES: PricingRuleType[] = [
  "season",
  "day_of_week",
  "length_of_stay",
  "advance_purchase",
];
const MODIFIER_TYPES: PricingModifierType[] = ["percentage", "fixed_amount", "set_value"];
const DAYS = [
  { v: 1, l: "Mon" },
  { v: 2, l: "Tue" },
  { v: 3, l: "Wed" },
  { v: 4, l: "Thu" },
  { v: 5, l: "Fri" },
  { v: 6, l: "Sat" },
  { v: 7, l: "Sun" },
];

export default function PricingPage() {
  const { activeHotel } = useShell();
  const [rules, setRules] = useState<PricingRule[]>([]);
  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modal, setModal] = useState<PricingRule | "new" | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const [r, rt] = await Promise.all([
        Pricing.list(activeHotel.id),
        RoomTypes.list(activeHotel.id),
      ]);
      setRules(r.rules);
      setRoomTypes(rt.room_types);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [activeHotel.id]); // eslint-disable-line react-hooks/exhaustive-deps

  async function remove(rule: PricingRule) {
    if (!window.confirm(`${t("delete")} ${rule.name}?`)) return;
    try {
      await Pricing.remove(activeHotel.id, rule.id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    }
  }

  return (
    <div>
      <PageHeader
        title={t("nav_pricing")}
        description={activeHotel.name}
        actions={<Button onClick={() => setModal("new")}>{t("add")}</Button>}
      />

      <ErrorBanner message={error} />

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : rules.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div className="overflow-x-auto rounded-lg border border-neutral-200 bg-white">
          <table className="w-full min-w-[700px] text-sm">
            <thead>
              <tr className="border-b border-neutral-200 bg-neutral-50 text-left text-xs uppercase tracking-wide text-neutral-500">
                <th className="px-4 py-2.5">Name</th>
                <th>Type</th>
                <th>Modifier</th>
                <th>Range</th>
                <th>Priority</th>
                <th>Enabled</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rules.map((r) => (
                <tr key={r.id} className="border-b border-neutral-100 last:border-0">
                  <td className="px-4 py-2.5">{r.name}</td>
                  <td className="text-xs">{r.rule_type}</td>
                  <td className="text-xs font-mono">
                    {r.modifier_type} / {r.modifier_value}
                  </td>
                  <td className="text-xs">
                    {r.start_date || "—"} → {r.end_date || "—"}
                  </td>
                  <td className="text-xs">{r.priority}</td>
                  <td className="text-xs">{r.enabled ? "yes" : "no"}</td>
                  <td className="px-4 text-right">
                    <Button variant="ghost" onClick={() => setModal(r)}>
                      {t("edit")}
                    </Button>
                    <Button variant="ghost" onClick={() => remove(r)}>
                      {t("delete")}
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <RuleModal
        open={!!modal}
        onClose={() => setModal(null)}
        hotelID={activeHotel.id}
        roomTypes={roomTypes}
        rule={modal && modal !== "new" ? modal : null}
        onSaved={async () => {
          setModal(null);
          await load();
        }}
      />

      <div className="mt-8">
        <Card title="Availability overrides">
          <p className="text-sm text-neutral-600">
            Per-day overrides (close-outs, special rates, blocks) are edited on the
            calendar, where the cell grid mirrors the underlying data shape. Open
            the calendar and toggle{" "}
            <span className="font-medium text-neutral-800">{t("bulk_edit")}</span>{" "}
            to select cells and apply changes.
          </p>
          <div className="mt-3">
            <Link href="/calendar" className="text-sm font-medium text-neutral-900 underline">
              {t("nav_calendar")} →
            </Link>
          </div>
        </Card>
      </div>
    </div>
  );
}

function RuleModal({
  open,
  onClose,
  hotelID,
  roomTypes,
  rule,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  hotelID: string;
  roomTypes: RoomType[];
  rule: PricingRule | null;
  onSaved: () => Promise<void>;
}) {
  const [form, setForm] = useState<CreatePricingRuleRequest>({
    name: "",
    rule_type: "season",
    modifier_type: "percentage",
    modifier_value: "10",
    enabled: true,
  });
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    if (rule) {
      setForm({
        name: rule.name,
        room_type_id: rule.room_type_id,
        rule_type: rule.rule_type,
        start_date: rule.start_date,
        end_date: rule.end_date,
        days_of_week: rule.days_of_week,
        modifier_type: rule.modifier_type,
        modifier_value: rule.modifier_value,
        min_nights: rule.min_nights,
        max_nights: rule.max_nights,
        min_days_ahead: rule.min_days_ahead,
        max_days_ahead: rule.max_days_ahead,
        priority: rule.priority,
        enabled: rule.enabled,
      });
    } else {
      setForm({
        name: "",
        rule_type: "season",
        modifier_type: "percentage",
        modifier_value: "10",
        enabled: true,
      });
    }
    setErr(null);
  }, [open, rule]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setErr(null);
    try {
      if (rule) {
        // room_type_id is intentionally not updatable per the API contract.
        const patch = { ...form };
        delete patch.room_type_id;
        await Pricing.update(hotelID, rule.id, patch);
      } else {
        await Pricing.create(hotelID, form);
      }
      await onSaved();
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  function toggleDay(day: number) {
    const arr = form.days_of_week || [];
    const next = arr.includes(day) ? arr.filter((d) => d !== day) : [...arr, day].sort();
    setForm({ ...form, days_of_week: next });
  }

  return (
    <Modal open={open} onClose={onClose} title={rule ? t("edit") : t("add")}>
      <form onSubmit={submit} className="space-y-3">
        <Field label="Name">
          <TextInput
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Rule type">
            <Select
              value={form.rule_type}
              onChange={(e) =>
                setForm({ ...form, rule_type: e.target.value as PricingRuleType })
              }
            >
              {RULE_TYPES.map((rt) => (
                <option key={rt} value={rt}>
                  {rt}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Room type (optional)">
            <Select
              value={form.room_type_id || ""}
              onChange={(e) =>
                setForm({ ...form, room_type_id: e.target.value || null })
              }
              disabled={!!rule}
            >
              <option value="">All</option>
              {roomTypes.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Modifier type">
            <Select
              value={form.modifier_type}
              onChange={(e) =>
                setForm({ ...form, modifier_type: e.target.value as PricingModifierType })
              }
            >
              {MODIFIER_TYPES.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Modifier value">
            <TextInput
              required
              value={form.modifier_value}
              onChange={(e) => setForm({ ...form, modifier_value: e.target.value })}
            />
          </Field>
          <Field label="Start date">
            <TextInput
              type="date"
              value={form.start_date || ""}
              onChange={(e) => setForm({ ...form, start_date: e.target.value || null })}
            />
          </Field>
          <Field label="End date">
            <TextInput
              type="date"
              value={form.end_date || ""}
              onChange={(e) => setForm({ ...form, end_date: e.target.value || null })}
            />
          </Field>
          <Field label="Min nights">
            <TextInput
              type="number"
              min={1}
              value={form.min_nights ?? ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  min_nights: e.target.value ? parseInt(e.target.value, 10) : null,
                })
              }
            />
          </Field>
          <Field label="Priority">
            <TextInput
              type="number"
              value={form.priority ?? 0}
              onChange={(e) => setForm({ ...form, priority: parseInt(e.target.value, 10) })}
            />
          </Field>
        </div>
        <Field label="Days of week (Mon=1, Sun=7)">
          <div className="flex flex-wrap gap-2">
            {DAYS.map((d) => (
              <button
                key={d.v}
                type="button"
                onClick={() => toggleDay(d.v)}
                className={`rounded-md border px-2 py-1 text-xs ${
                  form.days_of_week?.includes(d.v)
                    ? "border-neutral-900 bg-neutral-900 text-white"
                    : "border-neutral-300 bg-white"
                }`}
              >
                {d.l}
              </button>
            ))}
          </div>
        </Field>
        <label className="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={form.enabled ?? true}
            onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
          />
          Enabled
        </label>
        <ErrorBanner message={err} />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button type="submit" loading={saving}>
            {t("save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
