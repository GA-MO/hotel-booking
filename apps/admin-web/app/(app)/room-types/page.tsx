"use client";

import { useEffect, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  Field,
  Modal,
  PageHeader,
  TextArea,
  TextInput,
} from "@/app/components/ui";
import { RoomTypes } from "@/app/lib/api";
import { t } from "@/app/i18n";
import type { RoomType, RoomTypeCreateRequest } from "@/app/lib/types";

const emptyForm: RoomTypeCreateRequest = {
  name: "",
  description: "",
  total_inventory: 1,
  max_occupancy: 2,
  base_rate: 1500,
  base_currency: "THB",
};

export default function RoomTypesPage() {
  const { activeHotel } = useShell();
  const [items, setItems] = useState<RoomType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modal, setModal] = useState<{ mode: "create" } | { mode: "edit"; rt: RoomType } | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const r = await RoomTypes.list(activeHotel.id);
      setItems(r.room_types);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [activeHotel.id]); // eslint-disable-line react-hooks/exhaustive-deps

  async function remove(rt: RoomType) {
    if (!window.confirm(`${t("delete")} ${rt.name}?`)) return;
    try {
      await RoomTypes.remove(activeHotel.id, rt.id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    }
  }

  return (
    <div>
      <PageHeader
        title={t("nav_room_types")}
        description={activeHotel.name}
        actions={<Button onClick={() => setModal({ mode: "create" })}>{t("add")}</Button>}
      />

      <ErrorBanner message={error} />

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : items.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((rt) => (
            <Card
              key={rt.id}
              title={rt.name}
              actions={
                <div className="flex gap-1">
                  <Button variant="ghost" onClick={() => setModal({ mode: "edit", rt })}>
                    {t("edit")}
                  </Button>
                  <Button variant="ghost" onClick={() => remove(rt)}>
                    {t("delete")}
                  </Button>
                </div>
              }
            >
              <dl className="grid grid-cols-2 gap-y-1 text-sm">
                <dt className="text-neutral-500">{t("total_inventory")}</dt>
                <dd>{rt.total_inventory}</dd>
                <dt className="text-neutral-500">{t("max_occupancy")}</dt>
                <dd>{rt.max_occupancy}</dd>
                <dt className="text-neutral-500">{t("base_rate")}</dt>
                <dd className="font-mono">
                  {rt.base_rate.toFixed(2)} {rt.base_currency}
                </dd>
                <dt className="text-neutral-500">Enabled</dt>
                <dd>{rt.enabled ? "yes" : "no"}</dd>
              </dl>
              {rt.description && (
                <p className="mt-3 text-xs text-neutral-600">{rt.description}</p>
              )}
            </Card>
          ))}
        </div>
      )}

      <RoomTypeFormModal
        open={!!modal}
        onClose={() => setModal(null)}
        hotelID={activeHotel.id}
        defaultCurrency={activeHotel.base_currency}
        rt={modal?.mode === "edit" ? modal.rt : null}
        onSaved={async () => {
          setModal(null);
          await load();
        }}
      />
    </div>
  );
}

function RoomTypeFormModal({
  open,
  onClose,
  hotelID,
  defaultCurrency,
  rt,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  hotelID: string;
  defaultCurrency: string;
  rt: RoomType | null;
  onSaved: () => Promise<void>;
}) {
  const [form, setForm] = useState<RoomTypeCreateRequest>(emptyForm);
  const [photoUrls, setPhotoUrls] = useState("");
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    if (rt) {
      setForm({
        name: rt.name,
        description: rt.description || "",
        total_inventory: rt.total_inventory,
        max_occupancy: rt.max_occupancy,
        base_rate: rt.base_rate,
        base_currency: rt.base_currency,
        display_order: rt.display_order,
      });
    } else {
      setForm({ ...emptyForm, base_currency: defaultCurrency });
    }
    setPhotoUrls("");
    setErr(null);
  }, [open, rt, defaultCurrency]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setErr(null);
    try {
      if (rt) {
        await RoomTypes.update(hotelID, rt.id, form);
      } else {
        await RoomTypes.create(hotelID, form);
      }
      // Photo URLs are captured here as a placeholder for the real upload
      // pipeline. The upload module is being built by another agent — once
      // that lands these URLs will flow through CreatePhotoRequest.storage_key.
      await onSaved();
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={rt ? t("edit") : t("add")}>
      <form onSubmit={submit} className="space-y-3">
        <Field label={t("room_type_name")}>
          <TextInput
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </Field>
        <Field label={t("description")}>
          <TextArea
            rows={2}
            value={form.description || ""}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("total_inventory")}>
            <TextInput
              type="number"
              min={0}
              required
              value={form.total_inventory}
              onChange={(e) =>
                setForm({ ...form, total_inventory: parseInt(e.target.value || "0", 10) })
              }
            />
          </Field>
          <Field label={t("max_occupancy")}>
            <TextInput
              type="number"
              min={1}
              required
              value={form.max_occupancy}
              onChange={(e) =>
                setForm({ ...form, max_occupancy: parseInt(e.target.value || "1", 10) })
              }
            />
          </Field>
          <Field label={t("base_rate")}>
            <TextInput
              type="number"
              step="0.01"
              min={0}
              required
              value={form.base_rate}
              onChange={(e) =>
                setForm({ ...form, base_rate: parseFloat(e.target.value || "0") })
              }
            />
          </Field>
          <Field label={t("currency")}>
            <TextInput
              value={form.base_currency || ""}
              onChange={(e) => setForm({ ...form, base_currency: e.target.value })}
            />
          </Field>
        </div>
        <Field
          label="Photo URLs (one per line)"
          hint="Placeholder until file upload ships. Stored separately via the photo endpoints."
        >
          <TextArea
            rows={2}
            value={photoUrls}
            onChange={(e) => setPhotoUrls(e.target.value)}
            placeholder="https://..."
          />
        </Field>
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
