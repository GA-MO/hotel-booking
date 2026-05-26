"use client";

import { useEffect, useState } from "react";

import {
  ActionIcon,
  Button,
  Card,
  Center,
  Checkbox,
  ColorInput,
  Group,
  Loader,
  NativeSelect,
  Paper,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";

import { useShell } from "@/app/components/AppShell";
import SingleImageUpload from "@/app/components/SingleImageUpload";
import { ErrorBanner, PageHeader, StatusBadge } from "@/app/components/ui";
import { Landing, ApiClientError } from "@/app/lib/api";
import { notifySuccess } from "@/app/lib/notify";
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

const DEFAULT_SECTION_TYPES = [
  "hero",
  "gallery",
  "about",
  "rooms",
  "amenities",
  "location",
];

type DraftForm = {
  branding: Branding;
  seo: SEO;
  tracking: Tracking;
  sections: LandingSection[];
};

function emptyDraft(): DraftForm {
  return {
    branding: {
      primary_color: "#0f172a",
      accent_color: "#f59e0b",
      font_family: "Inter",
    },
    seo: {},
    tracking: {},
    sections: [
      {
        type: "hero",
        enabled: true,
        order: 0,
        content: { headline: "", subheadline: "" },
      },
    ],
  };
}

const TRACKING_FIELDS: Array<[keyof Tracking, string]> = [
  ["facebook_pixel_id", "Facebook Pixel"],
  ["google_analytics_id", "Google Analytics"],
  ["google_ads_conversion_id", "Google Ads Conversion"],
  ["gtm_id", "GTM"],
  ["line_tag_id", "LINE Tag"],
  ["tiktok_pixel_id", "TikTok Pixel"],
];

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
      notifySuccess(t("saved"));
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
      notifySuccess(
        p.status === "published" ? t("publish") + " ✓" : t("unpublish") + " ✓"
      );
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
    <div style={{ maxWidth: 1024 }}>
      <PageHeader
        title={t("nav_landing")}
        description={activeHotel.name}
        actions={
          <Group gap="xs">
            <NativeSelect
              value={locale}
              onChange={(e) => setLocale(e.currentTarget.value as Locale)}
              data={LOCALES.map((l) => ({ value: l, label: l.toUpperCase() }))}
              w={88}
              size="sm"
            />
            {page && <StatusBadge value={page.status} />}
          </Group>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      ) : (
        <Stack gap="md">
          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Branding
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <Stack gap={4}>
                <Text size="sm" fw={500} c="gray.7">
                  {t("logo_image")}
                </Text>
                <SingleImageUpload
                  hotelID={activeHotel.id}
                  kind="hotel_photo"
                  value={draft.branding.logo_url || ""}
                  onChange={(url) =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, logo_url: url },
                    })
                  }
                  onRemove={() =>
                    setDraft({
                      ...draft,
                      branding: { ...draft.branding, logo_url: "" },
                    })
                  }
                />
              </Stack>
              <TextInput
                label="Font family"
                value={draft.branding.font_family || ""}
                onChange={(e) =>
                  setDraft({
                    ...draft,
                    branding: {
                      ...draft.branding,
                      font_family: e.currentTarget.value,
                    },
                  })
                }
              />
              <ColorInput
                label="Primary color"
                value={draft.branding.primary_color || ""}
                onChange={(v) =>
                  setDraft({
                    ...draft,
                    branding: { ...draft.branding, primary_color: v },
                  })
                }
                format="hex"
              />
              <ColorInput
                label="Accent color"
                value={draft.branding.accent_color || ""}
                onChange={(v) =>
                  setDraft({
                    ...draft,
                    branding: { ...draft.branding, accent_color: v },
                  })
                }
                format="hex"
              />
            </SimpleGrid>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              SEO
            </Title>
            <Stack gap="sm">
              <TextInput
                label={t("landing_title")}
                value={draft.seo.title || ""}
                onChange={(e) =>
                  setDraft({
                    ...draft,
                    seo: { ...draft.seo, title: e.currentTarget.value },
                  })
                }
              />
              <Textarea
                label="Meta description"
                rows={2}
                value={draft.seo.description || ""}
                onChange={(e) =>
                  setDraft({
                    ...draft,
                    seo: { ...draft.seo, description: e.currentTarget.value },
                  })
                }
              />
              <Stack gap={4}>
                <Text size="sm" fw={500} c="gray.7">
                  {t("og_image")}
                </Text>
                <SingleImageUpload
                  hotelID={activeHotel.id}
                  kind="hotel_photo"
                  value={draft.seo.og_image_url || ""}
                  onChange={(url) =>
                    setDraft({
                      ...draft,
                      seo: { ...draft.seo, og_image_url: url },
                    })
                  }
                  onRemove={() =>
                    setDraft({
                      ...draft,
                      seo: { ...draft.seo, og_image_url: "" },
                    })
                  }
                />
              </Stack>
            </Stack>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Group justify="space-between" mb="sm">
              <Title order={3} size="h5">
                Sections
              </Title>
              <Button variant="default" size="sm" onClick={addSection}>
                {t("add")}
              </Button>
            </Group>
            <Stack gap="sm">
              {draft.sections.map((s, idx) => (
                <Paper key={idx} withBorder radius="md" p="sm" bg="gray.0">
                  <Group gap="xs" mb="xs" wrap="wrap">
                    <NativeSelect
                      value={s.type}
                      onChange={(e) => {
                        const next = [...draft.sections];
                        next[idx] = { ...s, type: e.currentTarget.value };
                        setDraft({ ...draft, sections: next });
                      }}
                      data={DEFAULT_SECTION_TYPES}
                      w={160}
                      size="xs"
                    />
                    <Checkbox
                      label="Enabled"
                      checked={s.enabled}
                      onChange={(e) => {
                        const next = [...draft.sections];
                        next[idx] = { ...s, enabled: e.currentTarget.checked };
                        setDraft({ ...draft, sections: next });
                      }}
                      size="xs"
                    />
                    <Group gap={4} ml="auto">
                      <ActionIcon
                        variant="subtle"
                        color="gray"
                        onClick={() => moveSection(idx, -1)}
                        disabled={idx === 0}
                        aria-label="Move up"
                      >
                        ↑
                      </ActionIcon>
                      <ActionIcon
                        variant="subtle"
                        color="gray"
                        onClick={() => moveSection(idx, 1)}
                        disabled={idx === draft.sections.length - 1}
                        aria-label="Move down"
                      >
                        ↓
                      </ActionIcon>
                      <ActionIcon
                        variant="subtle"
                        color="red"
                        onClick={() => removeSection(idx)}
                        aria-label={t("remove")}
                      >
                        ✕
                      </ActionIcon>
                    </Group>
                  </Group>
                  <Textarea
                    rows={3}
                    autosize
                    minRows={3}
                    value={JSON.stringify(s.content || {}, null, 2)}
                    onChange={(e) => {
                      try {
                        const parsed = JSON.parse(e.currentTarget.value || "{}");
                        const next = [...draft.sections];
                        next[idx] = { ...s, content: parsed };
                        setDraft({ ...draft, sections: next });
                      } catch {
                        // ignore until parseable
                      }
                    }}
                    styles={{ input: { fontFamily: "monospace", fontSize: 12 } }}
                  />
                </Paper>
              ))}
            </Stack>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Tracking pixels
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              {TRACKING_FIELDS.map(([k, label]) => (
                <TextInput
                  key={k}
                  label={label}
                  value={(draft.tracking[k] as string) || ""}
                  onChange={(e) =>
                    setDraft({
                      ...draft,
                      tracking: {
                        ...draft.tracking,
                        [k]: e.currentTarget.value,
                      },
                    })
                  }
                />
              ))}
            </SimpleGrid>
          </Card>

          <Group justify="flex-end">
            {page && (
              <Button
                variant="default"
                onClick={publishToggle}
                loading={saving}
              >
                {page.status === "published" ? t("unpublish") : t("publish")}
              </Button>
            )}
            <Button onClick={save} loading={saving} color="dark">
              {t("save")}
            </Button>
          </Group>
        </Stack>
      )}
    </div>
  );
}
