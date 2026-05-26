"use client";

import { useEffect, useState } from "react";

import { useShell } from "@/app/components/AppShell";
import {
  Button,
  Card,
  ErrorBanner,
  PageHeader,
  Select,
  StatusBadge,
} from "@/app/components/ui";
import { Subs } from "@/app/lib/api";
import { centsToDisplay } from "@/app/lib/money";
import { getLocale, setLocale, type Locale } from "@/app/i18n";
import { t } from "@/app/i18n";
import type { Subscription } from "@/app/lib/types";

export default function SettingsPage() {
  const { activeHotel } = useShell();
  const [sub, setSub] = useState<Subscription | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [locale, setLocaleState] = useState<Locale>("th");

  useEffect(() => {
    setLocaleState(getLocale());
    Subs.get()
      .then(setSub)
      .catch((e) => setError(e instanceof Error ? e.message : t("error_generic")))
      .finally(() => setLoading(false));
  }, []);

  function changeLocale(l: Locale) {
    setLocale(l);
    setLocaleState(l);
    window.location.reload();
  }

  return (
    <div className="max-w-3xl">
      <PageHeader title={t("nav_settings")} description={activeHotel.name} />

      <ErrorBanner message={error} />

      <Card title="Language">
        <Select
          value={locale}
          onChange={(e) => changeLocale(e.target.value as Locale)}
          className="w-40"
        >
          <option value="th">ไทย</option>
          <option value="en">English</option>
        </Select>
      </Card>

      <div className="h-4" />

      <Card title={t("subscription_status")}>
        {loading ? (
          <p className="text-sm text-neutral-500">{t("loading")}</p>
        ) : sub ? (
          <>
            <div className="flex items-center gap-2">
              <StatusBadge value={sub.status} />
              {sub.plan_code && <span className="text-sm">{sub.plan_code}</span>}
            </div>
            <dl className="mt-4 grid grid-cols-2 gap-y-2 text-sm">
              {sub.trial_ends_at && (
                <>
                  <dt className="text-neutral-500">Trial ends</dt>
                  <dd>{new Date(sub.trial_ends_at).toLocaleDateString()}</dd>
                </>
              )}
              {sub.current_period_end && (
                <>
                  <dt className="text-neutral-500">Period ends</dt>
                  <dd>{new Date(sub.current_period_end).toLocaleDateString()}</dd>
                </>
              )}
              {sub.unit_price_cents != null && sub.currency && (
                <>
                  <dt className="text-neutral-500">Unit price</dt>
                  <dd className="font-mono">
                    {centsToDisplay(sub.unit_price_cents, sub.currency)}
                  </dd>
                </>
              )}
              {sub.payment_method_last4 && (
                <>
                  <dt className="text-neutral-500">Card</dt>
                  <dd>
                    {sub.payment_method_brand} •••• {sub.payment_method_last4}
                  </dd>
                </>
              )}
            </dl>
            <p className="mt-4 text-xs text-neutral-500">
              Payment capture (Stripe / Omise) is gated behind Phase 3 wiring; status here
              reflects the backend state machine only.
            </p>
            {sub.status !== "cancelled" && sub.status !== "terminated" && (
              <div className="mt-4">
                <Button
                  variant="danger"
                  onClick={async () => {
                    const reason = window.prompt("Reason?") || "";
                    if (!window.confirm("Cancel subscription?")) return;
                    try {
                      const updated = await Subs.cancel(reason);
                      setSub(updated);
                    } catch (e) {
                      setError(e instanceof Error ? e.message : t("error_generic"));
                    }
                  }}
                >
                  Cancel subscription
                </Button>
              </div>
            )}
          </>
        ) : (
          <p className="text-sm text-neutral-500">{t("no_data")}</p>
        )}
      </Card>
    </div>
  );
}
