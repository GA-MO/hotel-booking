"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { Hotels, Landing, RoomTypes } from "@/app/lib/api";
import {
  isAuthed,
  markOnboardingDone,
  setActiveHotelID,
} from "@/app/lib/auth";
import {
  Button,
  Card,
  ErrorBanner,
  Field,
  Select,
  TextArea,
  TextInput,
} from "@/app/components/ui";
import { t } from "@/app/i18n";
import type { Hotel } from "@/app/lib/types";

const TIMEZONES = [
  "Asia/Bangkok",
  "Asia/Singapore",
  "Asia/Kuala_Lumpur",
  "Asia/Jakarta",
];
const CURRENCIES = ["THB", "USD", "SGD"];

type Step = 1 | 2 | 3;

export default function OnboardingPage() {
  const router = useRouter();
  const [step, setStep] = useState<Step>(1);
  const [hotel, setHotel] = useState<Hotel | null>(null);
  const [error, setError] = useState<string | null>(null);

  // step 1 — hotel basics
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [city, setCity] = useState("");
  const [country, setCountry] = useState("TH");
  const [timezone, setTimezone] = useState("Asia/Bangkok");
  const [currency, setCurrency] = useState("THB");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");

  // step 2 — room type
  const [rtName, setRtName] = useState("");
  const [rtInventory, setRtInventory] = useState(5);
  const [rtMaxOccupancy, setRtMaxOccupancy] = useState(2);
  const [rtRate, setRtRate] = useState(1500);

  // step 3 — landing
  const [pageTitle, setPageTitle] = useState("");
  const [heroText, setHeroText] = useState("");
  const [pageLocale, setPageLocale] = useState("th");

  const [submitting, setSubmitting] = useState(false);

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

  async function submitStep1(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const created = await Hotels.create({
        slug,
        name,
        country,
        timezone,
        base_currency: currency,
      });
      const updated = await Hotels.update(created.id, {
        city,
        phone,
        address_line: address,
      }).catch(() => created);
      setActiveHotelID(updated.id);
      setHotel(updated);
      setStep(2);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitStep2(e: React.FormEvent) {
    e.preventDefault();
    if (!hotel) return;
    setSubmitting(true);
    setError(null);
    try {
      await RoomTypes.create(hotel.id, {
        name: rtName,
        total_inventory: rtInventory,
        max_occupancy: rtMaxOccupancy,
        base_rate: rtRate,
        base_currency: hotel.base_currency,
      });
      setStep(3);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  async function submitStep3(e: React.FormEvent) {
    e.preventDefault();
    if (!hotel) return;
    setSubmitting(true);
    setError(null);
    try {
      await Landing.upsert(hotel.id, pageLocale, {
        branding: { primary_color: "#0f172a", accent_color: "#f59e0b" },
        seo: { title: pageTitle },
        tracking: {},
        sections: [
          {
            type: "hero",
            enabled: true,
            order: 0,
            content: { headline: pageTitle, subheadline: heroText },
          },
        ],
      });
      markOnboardingDone();
      router.replace("/");
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSubmitting(false);
    }
  }

  function StepDot({ n }: { n: Step }) {
    const active = step === n;
    const done = step > n;
    return (
      <div className="flex items-center gap-2">
        <span
          className={`flex h-6 w-6 items-center justify-center rounded-full text-xs font-medium ${
            done
              ? "bg-emerald-600 text-white"
              : active
                ? "bg-neutral-900 text-white"
                : "bg-neutral-200 text-neutral-600"
          }`}
        >
          {done ? "✓" : n}
        </span>
        <span className={`text-sm ${active ? "font-semibold" : "text-neutral-500"}`}>
          {n === 1 && t("onboarding_step1")}
          {n === 2 && t("onboarding_step2")}
          {n === 3 && t("onboarding_step3")}
        </span>
      </div>
    );
  }

  return (
    <main className="mx-auto min-h-screen max-w-2xl px-4 py-10">
      <h1 className="mb-2 text-2xl font-semibold">{t("onboarding_title")}</h1>
      <div className="mb-6 flex flex-wrap gap-x-6 gap-y-2">
        <StepDot n={1} />
        <StepDot n={2} />
        <StepDot n={3} />
      </div>

      <ErrorBanner message={error} />

      {step === 1 && (
        <Card title={t("onboarding_step1")}>
          <form onSubmit={submitStep1} className="space-y-3">
            <Field label={t("hotel_name")}>
              <TextInput required value={name} onChange={(e) => setName(e.target.value)} />
            </Field>
            <Field label={t("slug")} hint={t("slug_hint")}>
              <TextInput
                required
                pattern="^[a-z0-9]([a-z0-9-]*[a-z0-9])?$"
                value={slug}
                onChange={(e) => setSlug(e.target.value.toLowerCase())}
              />
            </Field>
            <Field label={t("address")}>
              <TextArea
                rows={2}
                value={address}
                onChange={(e) => setAddress(e.target.value)}
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("city")}>
                <TextInput value={city} onChange={(e) => setCity(e.target.value)} />
              </Field>
              <Field label={t("country")}>
                <TextInput value={country} onChange={(e) => setCountry(e.target.value)} />
              </Field>
              <Field label={t("timezone")}>
                <Select value={timezone} onChange={(e) => setTimezone(e.target.value)}>
                  {TIMEZONES.map((tz) => (
                    <option key={tz} value={tz}>
                      {tz}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label={t("currency")}>
                <Select value={currency} onChange={(e) => setCurrency(e.target.value)}>
                  {CURRENCIES.map((c) => (
                    <option key={c} value={c}>
                      {c}
                    </option>
                  ))}
                </Select>
              </Field>
              <Field label={t("phone")}>
                <TextInput value={phone} onChange={(e) => setPhone(e.target.value)} />
              </Field>
            </div>
            <div className="flex justify-end">
              <Button type="submit" loading={submitting}>
                {t("next")}
              </Button>
            </div>
          </form>
        </Card>
      )}

      {step === 2 && (
        <Card title={t("onboarding_step2")}>
          <form onSubmit={submitStep2} className="space-y-3">
            <Field label={t("room_type_name")}>
              <TextInput
                required
                placeholder="Standard / Deluxe / Suite"
                value={rtName}
                onChange={(e) => setRtName(e.target.value)}
              />
            </Field>
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("total_inventory")}>
                <TextInput
                  type="number"
                  min={1}
                  required
                  value={rtInventory}
                  onChange={(e) =>
                    setRtInventory(parseInt(e.target.value || "1", 10))
                  }
                />
              </Field>
              <Field label={t("max_occupancy")}>
                <TextInput
                  type="number"
                  min={1}
                  required
                  value={rtMaxOccupancy}
                  onChange={(e) =>
                    setRtMaxOccupancy(parseInt(e.target.value || "1", 10))
                  }
                />
              </Field>
              <Field label={t("base_rate")}>
                <TextInput
                  type="number"
                  step="0.01"
                  min={0}
                  required
                  value={rtRate}
                  onChange={(e) => setRtRate(parseFloat(e.target.value || "0"))}
                />
              </Field>
            </div>
            <div className="flex justify-between">
              <Button type="button" variant="secondary" onClick={() => setStep(1)}>
                {t("back")}
              </Button>
              <Button type="submit" loading={submitting}>
                {t("next")}
              </Button>
            </div>
          </form>
        </Card>
      )}

      {step === 3 && (
        <Card title={t("onboarding_step3")}>
          <form onSubmit={submitStep3} className="space-y-3">
            <Field label={t("locale")}>
              <Select value={pageLocale} onChange={(e) => setPageLocale(e.target.value)}>
                <option value="th">ไทย</option>
                <option value="en">English</option>
              </Select>
            </Field>
            <Field label={t("landing_title")}>
              <TextInput
                required
                value={pageTitle}
                onChange={(e) => setPageTitle(e.target.value)}
              />
            </Field>
            <Field label={t("hero_text")}>
              <TextArea
                rows={3}
                value={heroText}
                onChange={(e) => setHeroText(e.target.value)}
              />
            </Field>
            <div className="flex justify-between">
              <Button type="button" variant="secondary" onClick={() => setStep(2)}>
                {t("back")}
              </Button>
              <Button type="submit" loading={submitting}>
                {t("onboarding_done")}
              </Button>
            </div>
          </form>
        </Card>
      )}
    </main>
  );
}
