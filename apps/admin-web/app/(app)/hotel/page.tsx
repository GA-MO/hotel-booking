"use client";

import { useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  Card,
  ErrorBanner,
  Field,
  PageHeader,
  Select,
  StatusBadge,
  TextArea,
  TextInput,
} from "@/app/components/ui";
import { Hotels } from "@/app/lib/api";
import { t } from "@/app/i18n";
import type { Hotel, HotelUpdateRequest } from "@/app/lib/types";

const TIMEZONES = [
  "Asia/Bangkok",
  "Asia/Singapore",
  "Asia/Kuala_Lumpur",
  "Asia/Jakarta",
  "Asia/Ho_Chi_Minh",
  "Asia/Manila",
];
const CURRENCIES = ["THB", "USD", "SGD", "MYR", "IDR", "VND", "PHP"];

export default function HotelSettingsPage() {
  const { activeHotel, reloadHotels } = useShell();
  const [hotel, setHotel] = useState<Hotel>(activeHotel);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [savedAt, setSavedAt] = useState<number | null>(null);

  function patch<K extends keyof Hotel>(k: K, v: Hotel[K]) {
    setHotel((h) => ({ ...h, [k]: v }));
  }

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const req: HotelUpdateRequest = {
        name: hotel.name,
        description: hotel.description,
        address_line: hotel.address_line,
        city: hotel.city,
        country: hotel.country,
        postal_code: hotel.postal_code,
        phone: hotel.phone,
        email: hotel.email,
        line_id: hotel.line_id,
        timezone: hotel.timezone,
        base_currency: hotel.base_currency,
        check_in_time: hotel.check_in_time,
        check_out_time: hotel.check_out_time,
      };
      const updated = await Hotels.update(hotel.id, req);
      setHotel(updated);
      setSavedAt(Date.now());
      await reloadHotels();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="max-w-3xl">
      <PageHeader
        title={t("nav_hotel")}
        description={hotel.name}
        actions={
          <div className="flex items-center gap-2">
            <StatusBadge value={hotel.status} />
            <StatusBadge value={`kyc:${hotel.kyc_status}`} />
          </div>
        }
      />

      <ErrorBanner message={error} />
      {savedAt && (
        <p className="mb-3 text-xs text-emerald-700">{t("saved")}</p>
      )}

      <Card title="Basics">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("hotel_name")}>
            <TextInput value={hotel.name} onChange={(e) => patch("name", e.target.value)} />
          </Field>
          <Field label={t("slug")} hint={t("slug_hint")}>
            <TextInput value={hotel.slug} disabled />
          </Field>
          <Field label={t("description")} >
            <TextArea
              rows={3}
              value={hotel.description || ""}
              onChange={(e) => patch("description", e.target.value)}
            />
          </Field>
        </div>
      </Card>

      <div className="h-4" />

      <Card title="Location">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("address")}>
            <TextInput
              value={hotel.address_line || ""}
              onChange={(e) => patch("address_line", e.target.value)}
            />
          </Field>
          <Field label={t("city")}>
            <TextInput
              value={hotel.city || ""}
              onChange={(e) => patch("city", e.target.value)}
            />
          </Field>
          <Field label={t("country")}>
            <TextInput
              value={hotel.country || ""}
              onChange={(e) => patch("country", e.target.value)}
            />
          </Field>
          <Field label={t("postal_code")}>
            <TextInput
              value={hotel.postal_code || ""}
              onChange={(e) => patch("postal_code", e.target.value)}
            />
          </Field>
        </div>
      </Card>

      <div className="h-4" />

      <Card title="Contact">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("phone")}>
            <TextInput
              value={hotel.phone || ""}
              onChange={(e) => patch("phone", e.target.value)}
            />
          </Field>
          <Field label={t("email")}>
            <TextInput
              type="email"
              value={hotel.email || ""}
              onChange={(e) => patch("email", e.target.value)}
            />
          </Field>
          <Field label="LINE ID">
            <TextInput
              value={hotel.line_id || ""}
              onChange={(e) => patch("line_id", e.target.value)}
            />
          </Field>
        </div>
      </Card>

      <div className="h-4" />

      <Card title="Operations">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label={t("timezone")}>
            <Select
              value={hotel.timezone}
              onChange={(e) => patch("timezone", e.target.value)}
            >
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={t("currency")}>
            <Select
              value={hotel.base_currency}
              onChange={(e) => patch("base_currency", e.target.value)}
            >
              {CURRENCIES.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </Select>
          </Field>
          <Field label={t("check_in_time")}>
            <TextInput
              value={hotel.check_in_time}
              onChange={(e) => patch("check_in_time", e.target.value)}
            />
          </Field>
          <Field label={t("check_out_time")}>
            <TextInput
              value={hotel.check_out_time}
              onChange={(e) => patch("check_out_time", e.target.value)}
            />
          </Field>
        </div>
      </Card>

      <div className="h-4" />

      <Card title="KYC">
        <p className="text-sm text-neutral-600">
          Status: <StatusBadge value={hotel.kyc_status} />
        </p>
        <p className="mt-2 text-xs text-neutral-500">
          File-upload UI for KYC documents lives behind the upload module; this Phase 1
          screen tracks status only. Submit documents to your account manager out-of-band.
        </p>
      </Card>

      <div className="mt-6 flex justify-end">
        <Button onClick={save} loading={saving}>
          {t("save")}
        </Button>
      </div>
    </div>
  );
}
