"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import {
  Button,
  Card,
  Container,
  Group,
  NativeSelect,
  NumberInput,
  Stack,
  Stepper,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";

import { Hotels, Landing, RoomTypes } from "@/app/lib/api";
import {
  isAuthed,
  markOnboardingDone,
  setActiveHotelID,
} from "@/app/lib/auth";
import { ErrorBanner } from "@/app/components/ui";
import { notifySuccess } from "@/app/lib/notify";
import { t } from "@/app/i18n";
import type { Hotel } from "@/app/lib/types";

const TIMEZONES = [
  "Asia/Bangkok",
  "Asia/Singapore",
  "Asia/Kuala_Lumpur",
  "Asia/Jakarta",
];
const CURRENCIES = ["THB", "USD", "SGD"];
const LOCALES = [
  { value: "th", label: "ไทย" },
  { value: "en", label: "English" },
];

const SLUG_RE = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/;

export default function OnboardingPage() {
  const router = useRouter();
  const [active, setActive] = useState(0);
  const [hotel, setHotel] = useState<Hotel | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const hotelForm = useForm({
    initialValues: {
      name: "",
      slug: "",
      city: "",
      country: "TH",
      timezone: "Asia/Bangkok",
      currency: "THB",
      phone: "",
      address: "",
    },
    validate: {
      name: (v) => (v.trim() ? null : t("required_field")),
      slug: (v) => (SLUG_RE.test(v) ? null : t("slug_hint")),
    },
  });

  const roomTypeForm = useForm({
    initialValues: {
      name: "",
      inventory: 5,
      maxOccupancy: 2,
      rate: 1500,
    },
    validate: {
      name: (v) => (v.trim() ? null : t("required_field")),
    },
  });

  const landingForm = useForm({
    initialValues: {
      locale: "th",
      title: "",
      heroText: "",
    },
    validate: {
      title: (v) => (v.trim() ? null : t("required_field")),
    },
  });

  useEffect(() => {
    if (!isAuthed()) {
      router.replace("/login");
      return;
    }
    Hotels.list()
      .then((r) => {
        if (r.hotels.length > 0) {
          setHotel(r.hotels[0]);
          setActiveHotelID(r.hotels[0].id);
        }
      })
      .catch(() => {});
  }, [router]);

  async function submitHotel(values: typeof hotelForm.values) {
    setSubmitting(true);
    setError(null);
    try {
      const created = await Hotels.create({
        slug: values.slug,
        name: values.name,
        country: values.country,
        timezone: values.timezone,
        base_currency: values.currency,
      });
      const updated = await Hotels.update(created.id, {
        city: values.city,
        phone: values.phone,
        address_line: values.address,
      }).catch(() => created);
      setActiveHotelID(updated.id);
      setHotel(updated);
      notifySuccess(t("onboarding_step1") + " ✓");
      setActive(1);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitRoomType(values: typeof roomTypeForm.values) {
    if (!hotel) return;
    setSubmitting(true);
    setError(null);
    try {
      await RoomTypes.create(hotel.id, {
        name: values.name,
        total_inventory: values.inventory,
        max_occupancy: values.maxOccupancy,
        base_rate: values.rate,
        base_currency: hotel.base_currency,
      });
      notifySuccess(t("onboarding_step2") + " ✓");
      setActive(2);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitLanding(values: typeof landingForm.values) {
    if (!hotel) return;
    setSubmitting(true);
    setError(null);
    try {
      await Landing.upsert(hotel.id, values.locale, {
        branding: { primary_color: "#0f172a", accent_color: "#f59e0b" },
        seo: { title: values.title },
        tracking: {},
        sections: [
          {
            type: "hero",
            enabled: true,
            order: 0,
            content: {
              headline: values.title,
              subheadline: values.heroText,
            },
          },
        ],
      });
      markOnboardingDone();
      notifySuccess(t("onboarding_done"));
      router.replace("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Container size="md" py="xl">
      <Title order={1} size="h2" mb="lg">
        {t("onboarding_title")}
      </Title>

      <Stepper
        active={active}
        onStepClick={(s) => {
          // Allow backward navigation only — forward requires submitting the step.
          if (s < active) setActive(s);
        }}
        mb="lg"
        allowNextStepsSelect={false}
      >
        <Stepper.Step label={t("onboarding_step1")}>
          <Card withBorder radius="md" padding="lg" mt="md">
            <form onSubmit={hotelForm.onSubmit(submitHotel)}>
              <Stack gap="sm">
                <TextInput
                  label={t("hotel_name")}
                  required
                  {...hotelForm.getInputProps("name")}
                />
                <TextInput
                  label={t("slug")}
                  description={t("slug_hint")}
                  required
                  {...hotelForm.getInputProps("slug")}
                  onChange={(e) =>
                    hotelForm.setFieldValue(
                      "slug",
                      e.currentTarget.value.toLowerCase()
                    )
                  }
                />
                <Textarea
                  label={t("address")}
                  rows={2}
                  {...hotelForm.getInputProps("address")}
                />
                <Group grow>
                  <TextInput
                    label={t("city")}
                    {...hotelForm.getInputProps("city")}
                  />
                  <TextInput
                    label={t("country")}
                    {...hotelForm.getInputProps("country")}
                  />
                </Group>
                <Group grow>
                  <NativeSelect
                    label={t("timezone")}
                    data={TIMEZONES}
                    {...hotelForm.getInputProps("timezone")}
                  />
                  <NativeSelect
                    label={t("currency")}
                    data={CURRENCIES}
                    {...hotelForm.getInputProps("currency")}
                  />
                </Group>
                <TextInput
                  label={t("phone")}
                  {...hotelForm.getInputProps("phone")}
                />
                <ErrorBanner message={error} />
                <Group justify="flex-end">
                  <Button type="submit" loading={submitting} color="dark">
                    {t("next")}
                  </Button>
                </Group>
              </Stack>
            </form>
          </Card>
        </Stepper.Step>

        <Stepper.Step label={t("onboarding_step2")}>
          <Card withBorder radius="md" padding="lg" mt="md">
            <form onSubmit={roomTypeForm.onSubmit(submitRoomType)}>
              <Stack gap="sm">
                <TextInput
                  label={t("room_type_name")}
                  placeholder="Standard / Deluxe / Suite"
                  required
                  {...roomTypeForm.getInputProps("name")}
                />
                <Group grow>
                  <NumberInput
                    label={t("total_inventory")}
                    min={1}
                    required
                    {...roomTypeForm.getInputProps("inventory")}
                  />
                  <NumberInput
                    label={t("max_occupancy")}
                    min={1}
                    required
                    {...roomTypeForm.getInputProps("maxOccupancy")}
                  />
                  <NumberInput
                    label={t("base_rate")}
                    min={0}
                    step={0.01}
                    decimalScale={2}
                    required
                    {...roomTypeForm.getInputProps("rate")}
                  />
                </Group>
                <ErrorBanner message={error} />
                <Group justify="space-between">
                  <Button
                    type="button"
                    variant="default"
                    onClick={() => setActive(0)}
                  >
                    {t("back")}
                  </Button>
                  <Button type="submit" loading={submitting} color="dark">
                    {t("next")}
                  </Button>
                </Group>
              </Stack>
            </form>
          </Card>
        </Stepper.Step>

        <Stepper.Step label={t("onboarding_step3")}>
          <Card withBorder radius="md" padding="lg" mt="md">
            <form onSubmit={landingForm.onSubmit(submitLanding)}>
              <Stack gap="sm">
                <NativeSelect
                  label={t("locale")}
                  data={LOCALES}
                  {...landingForm.getInputProps("locale")}
                />
                <TextInput
                  label={t("landing_title")}
                  required
                  {...landingForm.getInputProps("title")}
                />
                <Textarea
                  label={t("hero_text")}
                  rows={3}
                  {...landingForm.getInputProps("heroText")}
                />
                <ErrorBanner message={error} />
                <Group justify="space-between">
                  <Button
                    type="button"
                    variant="default"
                    onClick={() => setActive(1)}
                  >
                    {t("back")}
                  </Button>
                  <Button type="submit" loading={submitting} color="dark">
                    {t("onboarding_done")}
                  </Button>
                </Group>
              </Stack>
            </form>
          </Card>
        </Stepper.Step>
      </Stepper>
    </Container>
  );
}
