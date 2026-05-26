"use client";

import { useEffect, useState } from "react";

import {
  Button,
  Card,
  Center,
  Divider,
  Group,
  Loader,
  Modal,
  NativeSelect,
  Stack,
  Text,
  Textarea,
  Title,
} from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";

import { useShell } from "@/app/components/AppShell";
import { ErrorBanner, PageHeader, StatusBadge } from "@/app/components/ui";
import { Subs } from "@/app/lib/api";
import { centsToDisplay } from "@/app/lib/money";
import { notifySuccess } from "@/app/lib/notify";
import { getLocale, setLocale, t, type Locale } from "@/app/i18n";
import type { Subscription } from "@/app/lib/types";

export default function SettingsPage() {
  const { activeHotel } = useShell();
  const [sub, setSub] = useState<Subscription | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [locale, setLocaleState] = useState<Locale>("th");
  const [cancelOpened, cancelModal] = useDisclosure(false);
  const [cancelReason, setCancelReason] = useState("");
  const [cancelling, setCancelling] = useState(false);

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

  async function cancelSubscription() {
    setCancelling(true);
    try {
      const updated = await Subs.cancel(cancelReason);
      setSub(updated);
      cancelModal.close();
      notifySuccess("Subscription cancelled");
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setCancelling(false);
    }
  }

  return (
    <div style={{ maxWidth: 768 }}>
      <PageHeader title={t("nav_settings")} description={activeHotel.name} />

      <ErrorBanner message={error} />

      <Stack gap="md">
        <Card withBorder radius="md" padding="lg">
          <Title order={3} size="h5" mb="sm">
            Language
          </Title>
          <NativeSelect
            data={[
              { value: "th", label: "ไทย" },
              { value: "en", label: "English" },
            ]}
            value={locale}
            onChange={(e) => changeLocale(e.currentTarget.value as Locale)}
            w={160}
          />
        </Card>

        <Card withBorder radius="md" padding="lg">
          <Title order={3} size="h5" mb="sm">
            {t("subscription_status")}
          </Title>
          {loading ? (
            <Center py="md">
              <Loader size="sm" />
            </Center>
          ) : sub ? (
            <Stack gap="sm">
              <Group gap="sm">
                <StatusBadge value={sub.status} />
                {sub.plan_code && (
                  <Text size="sm" c="dimmed">
                    {sub.plan_code}
                  </Text>
                )}
              </Group>

              <Divider />

              <Stack gap={6}>
                {sub.trial_ends_at && (
                  <KeyValue
                    label="Trial ends"
                    value={new Date(sub.trial_ends_at).toLocaleDateString()}
                  />
                )}
                {sub.current_period_end && (
                  <KeyValue
                    label="Period ends"
                    value={new Date(sub.current_period_end).toLocaleDateString()}
                  />
                )}
                {sub.unit_price_cents != null && sub.currency && (
                  <KeyValue
                    label="Unit price"
                    value={centsToDisplay(sub.unit_price_cents, sub.currency)}
                    mono
                  />
                )}
                {sub.payment_method_last4 && (
                  <KeyValue
                    label="Card"
                    value={`${sub.payment_method_brand} •••• ${sub.payment_method_last4}`}
                  />
                )}
              </Stack>

              <Text size="xs" c="dimmed">
                Payment capture (Stripe / Omise) is gated behind Phase 3 wiring;
                status here reflects the backend state machine only.
              </Text>

              {sub.status !== "cancelled" && sub.status !== "terminated" && (
                <Group justify="flex-start">
                  <Button
                    color="red"
                    variant="light"
                    onClick={() => {
                      setCancelReason("");
                      cancelModal.open();
                    }}
                  >
                    Cancel subscription
                  </Button>
                </Group>
              )}
            </Stack>
          ) : (
            <Text size="sm" c="dimmed">
              {t("no_data")}
            </Text>
          )}
        </Card>
      </Stack>

      <Modal
        opened={cancelOpened}
        onClose={cancelModal.close}
        title="Cancel subscription"
        centered
        radius="md"
      >
        <Stack gap="md">
          <Text size="sm">
            Are you sure? This stops new billing periods. Existing data stays.
          </Text>
          <Textarea
            label="Reason"
            placeholder="(optional)"
            value={cancelReason}
            onChange={(e) => setCancelReason(e.currentTarget.value)}
            rows={3}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={cancelModal.close}>
              {t("back")}
            </Button>
            <Button color="red" loading={cancelling} onClick={cancelSubscription}>
              Cancel subscription
            </Button>
          </Group>
        </Stack>
      </Modal>
    </div>
  );
}

function KeyValue({
  label,
  value,
  mono,
}: {
  label: string;
  value: string;
  mono?: boolean;
}) {
  return (
    <Group justify="space-between">
      <Text size="sm" c="dimmed">
        {label}
      </Text>
      <Text size="sm" ff={mono ? "monospace" : undefined}>
        {value}
      </Text>
    </Group>
  );
}
