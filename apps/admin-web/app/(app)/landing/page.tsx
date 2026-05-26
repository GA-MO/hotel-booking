"use client";

import { useEffect, useState } from "react";

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
import { Landing, ApiClientError } from "@/app/lib/api";
import { t } from "@/app/i18n";
import type {
  Branding,
  LandingPage,
  LandingSection,
  LandingUpdateRequest,
  SEO,
  Tracking,
} from "@/app/lib/types";

const LOCALES = ["th", "en"] as const;
type Locale = (typeof LOCALES)[number];

const DEFAULT_SECTION_TYPES = ["hero", "gallery", "about", "rooms", "amenities", "location"];

type DraftForm = {
  branding: Branding;
  seo: SEO;
  tracking: Tracking;
  sections: LandingSection[];
};

function emptyDraft(): DraftForm {
  return {
    branding: { primary_color: "#0f172a", accent_color: "#f59e0b", font_family: "Inter" },
    seo: {},
    tracking: {},
    sections: [
      { type: "hero", enabled: true, order: 0, content: { headline: "", subheadline: "" } },
    ],
  };
}

export default function LandingPageEditor() {
  const { activeHotel } = useShell();
  const [locale, setLocale] = useState<Locale>("th");
  const [page, setPage] = useState<LandingPage | null>(null);
  const [draft, setDraft] = useState<DraftForm>(emptyDraft());
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setLoading(true);
    setError(null);
    try {
      const p = await Landing.get(activeHotel.id, locale);
      setPage(p);
      setDraft({
        branding: p.branding || {},
        seo: p.seo || {},
        tracking: p.tracking || {},
        sections: p.sections.length ? p.sections : emptyDraft().sections,
      });
    } catch (e) {
      if (e instanceof ApiClientError && e.status === 404) {
        setPage(null);
        setDraft(emptyDraft());
      } else {
        setError(e instanceof Error ? e.message : t("error_generic"));
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, [activeHotel.id, locale]); // eslint-disable-line react-hooks/exhaustive-deps

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const req: LandingUpdateRequest = {
        branding: draft.branding,
        seo: draft.seo,
        tracking: draft.tracking,
        sections: draft.sections.map((s, i) => ({ ...s, order: i })),
      };
      const p = await Landing.upsert(activeHotel.id, locale, req);
      setPage(p);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  async function publishToggle() {
    if (!page) return;
    setSaving(true);
    setError(null);
    try {
      const fn = page.status === "published" ? Landing.unpublish : Landing.publish;
      const p = await fn(activeHotel.id, locale);
      setPage(p);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  function moveSection(idx: number, delta: number) {
    const next = [...draft.sections];
    const j = idx + delta;
    if (j < 0 || j >= next.length) return;
    [next[idx], next[j]] = [next[j], next[idx]];
    setDraft({ ...draft, sections: next });
  }

  function addSection() {
    setDraft({
      ...draft,
      sections: [
        ...draft.sections,
        {
          type: "about",
          enabled: true,
          order: draft.sections.length,
          content: { body: "" },
        },
      ],
    });
  }

  function removeSection(idx: number) {
    const next = [...draft.sections];
    next.splice(idx, 1);
    setDraft({ ...draft, sections: next });
  }

  return (
    <div className="max-w-4xl">
      <PageHeader
        title={t("nav_landing")}
        description={activeHotel.name}
        actions={
          <>
            <Select
              value={locale}
              onChange={(e) => setLocale(e.target.value as Locale)}
              className="w-28"
            >
              {LOCALES.map((l) => (
                <option key={l} value={l}>
                  {l.toUpperCase()}
                </option>
              ))}
            </Select>
            {page && <StatusBadge value={page.status} />}
          </>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <p className="text-sm text-neutral-500">{t("loading")}</p>
      ) : (
        <>
          <Card title="Branding">
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Logo URL">
                <TextInput
                  value={draft.branding.logo_url || ""}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, logo_url: e.target.value },
                    })
                  }
                />
              </Field>
              <Field label="Font family">
                <TextInput
                  value={draft.branding.font_family || ""}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, font_family: e.target.value },
                    })
                  }
                />
              </Field>
              <Field label="Primary color (#hex)">
                <TextInput
                  value={draft.branding.primary_color || ""}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, primary_color: e.target.value },
                    })
                  }
                />
              </Field>
              <Field label="Accent color (#hex)">
                <TextInput
                  value={draft.branding.accent_color || ""}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, accent_color: e.target.value },
                    })
                  }
                />
              </Field>
            </div>
          </Card>

          <div className="h-4" />

          <Card title="SEO">
            <div className="grid gap-3">
              <Field label={t("landing_title")}>
                <TextInput
                  value={draft.seo.title || ""}
                  onChange={(e) =>
                    setDraft({ ...draft, seo: { ...draft.seo, title: e.target.value } })
                  }
                />
              </Field>
              <Field label="Meta description">
                <TextArea
                  rows={2}
                  value={draft.seo.description || ""}
                  onChange={(e) =>
                    setDraft({ ...draft, seo: { ...draft.seo, description: e.target.value } })
                  }
                />
              </Field>
              <Field label="OG image URL">
                <TextInput
                  value={draft.seo.og_image_url || ""}
                  onChange={(e) =>
                    setDraft({ ...draft, seo: { ...draft.seo, og_image_url: e.target.value } })
                  }
                />
              </Field>
            </div>
          </Card>

          <div className="h-4" />

          <Card title="Sections" actions={<Button variant="secondary" onClick={addSection}>{t("add")}</Button>}>
            <div className="space-y-3">
              {draft.sections.map((s, idx) => (
                <div
                  key={idx}
                  className="rounded-md border border-neutral-200 bg-neutral-50 p-3"
                >
                  <div className="mb-2 flex items-center gap-2">
                    <Select
                      value={s.type}
                      onChange={(e) => {
                        const next = [...draft.sections];
                        next[idx] = { ...s, type: e.target.value };
                        setDraft({ ...draft, sections: next });
                      }}
                      className="w-40"
                    >
                      {DEFAULT_SECTION_TYPES.map((typ) => (
                        <option key={typ} value={typ}>
                          {typ}
                        </option>
                      ))}
                    </Select>
                    <label className="ml-2 inline-flex items-center gap-1 text-xs">
                      <input
                        type="checkbox"
                        checked={s.enabled}
                        onChange={(e) => {
                          const next = [...draft.sections];
                          next[idx] = { ...s, enabled: e.target.checked };
                          setDraft({ ...draft, sections: next });
                        }}
                      />
                      Enabled
                    </label>
                    <div className="ml-auto flex gap-1">
                      <Button variant="ghost" onClick={() => moveSection(idx, -1)}>↑</Button>
                      <Button variant="ghost" onClick={() => moveSection(idx, 1)}>↓</Button>
                      <Button variant="ghost" onClick={() => removeSection(idx)}>
                        {t("remove")}
                      </Button>
                    </div>
                  </div>
                  <TextArea
                    rows={3}
                    value={JSON.stringify(s.content || {}, null, 2)}
                    onChange={(e) => {
                      try {
                        const parsed = JSON.parse(e.target.value || "{}");
                        const next = [...draft.sections];
                        next[idx] = { ...s, content: parsed };
                        setDraft({ ...draft, sections: next });
                      } catch {
                        // ignore until parseable
                      }
                    }}
                  />
                </div>
              ))}
            </div>
          </Card>

          <div className="h-4" />

          <Card title="Tracking pixels">
            <div className="grid gap-3 sm:grid-cols-2">
              {(
                [
                  ["facebook_pixel_id", "Facebook Pixel"],
                  ["google_analytics_id", "Google Analytics"],
                  ["google_ads_conversion_id", "Google Ads Conversion"],
                  ["gtm_id", "GTM"],
                  ["line_tag_id", "LINE Tag"],
                  ["tiktok_pixel_id", "TikTok Pixel"],
                ] as Array<[keyof Tracking, string]>
              ).map(([k, label]) => (
                <Field key={k} label={label}>
                  <TextInput
                    value={(draft.tracking[k] as string) || ""}
                    onChange={(e) =>
                      setDraft({
                        ...draft,
                        tracking: { ...draft.tracking, [k]: e.target.value },
                      })
                    }
                  />
                </Field>
              ))}
            </div>
          </Card>

          <div className="mt-6 flex justify-end gap-2">
            {page && (
              <Button variant="secondary" onClick={publishToggle} loading={saving}>
                {page.status === "published" ? t("unpublish") : t("publish")}
              </Button>
            )}
            <Button onClick={save} loading={saving}>
              {t("save")}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
