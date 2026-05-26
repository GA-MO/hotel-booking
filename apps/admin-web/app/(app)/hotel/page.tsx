"use client";

import { useState } from "react";

import {
  Button,
  Card,
  Group,
  Modal,
  NativeSelect,
  SimpleGrid,
  Stack,
  Text,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { useDisclosure } from "@mantine/hooks";

import { useShell } from "@/app/components/AppShell";
import {
  ErrorBanner,
  PageHeader,
  StatusBadge,
} from "@/app/components/ui";
import { Hotels } from "@/app/lib/api";
import { notifySuccess } from "@/app/lib/notify";
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
  const [goingLive, setGoingLive] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [confirmOpened, confirmModal] = useDisclosure(false);

  const form = useForm({
    initialValues: {
      name: hotel.name,
      description: hotel.description || "",
      address_line: hotel.address_line || "",
      city: hotel.city || "",
      country: hotel.country || "",
      postal_code: hotel.postal_code || "",
      phone: hotel.phone || "",
      email: hotel.email || "",
      line_id: hotel.line_id || "",
      timezone: hotel.timezone,
      base_currency: hotel.base_currency,
      check_in_time: hotel.check_in_time,
      check_out_time: hotel.check_out_time,
      promptpay_id: hotel.promptpay_id || "",
    },
  });

  async function goLive() {
    setGoingLive(true);
    setError(null);
    confirmModal.close();
    try {
      const updated = await Hotels.goLive(hotel.id);
      setHotel(updated);
      notifySuccess(t("go_live") + " ✓");
      await reloadHotels();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setGoingLive(false);
    }
  }

  async function save(values: typeof form.values) {
    setSaving(true);
    setError(null);
    try {
      const req: HotelUpdateRequest = {
        name: values.name,
        description: values.description,
        address_line: values.address_line,
        city: values.city,
        country: values.country,
        postal_code: values.postal_code,
        phone: values.phone,
        email: values.email,
        line_id: values.line_id,
        timezone: values.timezone,
        base_currency: values.base_currency,
        check_in_time: values.check_in_time,
        check_out_time: values.check_out_time,
        promptpay_id: values.promptpay_id || null,
      };
      const updated = await Hotels.update(hotel.id, req);
      setHotel(updated);
      form.setInitialValues(values);
      form.resetDirty();
      notifySuccess(t("saved"));
      await reloadHotels();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div style={{ maxWidth: 768 }}>
      <PageHeader
        title={t("nav_hotel")}
        description={hotel.name}
        actions={
          <Group gap="xs">
            <StatusBadge value={hotel.status} />
            <StatusBadge value={`kyc:${hotel.kyc_status}`} />
            {hotel.status === "test" && (
              <Button
                onClick={confirmModal.open}
                loading={goingLive}
                color="dark"
                size="sm"
              >
                {t("go_live")}
              </Button>
            )}
          </Group>
        }
      />

      <ErrorBanner message={error} />

      <form onSubmit={form.onSubmit(save)}>
        <Stack gap="md">
          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Basics
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <TextInput
                label={t("hotel_name")}
                {...form.getInputProps("name")}
              />
              <TextInput
                label={t("slug")}
                description={t("slug_hint")}
                value={hotel.slug}
                disabled
              />
              <Textarea
                label={t("description")}
                rows={3}
                {...form.getInputProps("description")}
                style={{ gridColumn: "1 / -1" }}
              />
            </SimpleGrid>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Location
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <TextInput
                label={t("address")}
                {...form.getInputProps("address_line")}
              />
              <TextInput
                label={t("city")}
                {...form.getInputProps("city")}
              />
              <TextInput
                label={t("country")}
                {...form.getInputProps("country")}
              />
              <TextInput
                label={t("postal_code")}
                {...form.getInputProps("postal_code")}
              />
            </SimpleGrid>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Contact
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <TextInput
                label={t("phone")}
                {...form.getInputProps("phone")}
              />
              <TextInput
                label={t("email")}
                type="email"
                {...form.getInputProps("email")}
              />
              <TextInput
                label="LINE ID"
                {...form.getInputProps("line_id")}
              />
            </SimpleGrid>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              Operations
            </Title>
            <SimpleGrid cols={{ base: 1, sm: 2 }} spacing="sm">
              <NativeSelect
                label={t("timezone")}
                data={TIMEZONES}
                {...form.getInputProps("timezone")}
              />
              <NativeSelect
                label={t("currency")}
                data={CURRENCIES}
                {...form.getInputProps("base_currency")}
              />
              <TextInput
                label={t("check_in_time")}
                {...form.getInputProps("check_in_time")}
              />
              <TextInput
                label={t("check_out_time")}
                {...form.getInputProps("check_out_time")}
              />
            </SimpleGrid>
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              {t("payment_section")}
            </Title>
            <TextInput
              label={t("promptpay_id")}
              description={t("promptpay_hint")}
              placeholder="0812345678"
              {...form.getInputProps("promptpay_id")}
            />
          </Card>

          <Card withBorder radius="md" padding="lg">
            <Title order={3} size="h5" mb="sm">
              KYC
            </Title>
            <Group gap="sm">
              <Text size="sm" c="dimmed">
                Status:
              </Text>
              <StatusBadge value={hotel.kyc_status} />
            </Group>
            <Text size="xs" c="dimmed" mt="xs">
              File-upload UI for KYC documents lives behind the upload module;
              this Phase 1 screen tracks status only. Submit documents to your
              account manager out-of-band.
            </Text>
          </Card>

          <Group justify="flex-end">
            <Button
              type="submit"
              loading={saving}
              disabled={!form.isDirty()}
              color="dark"
            >
              {t("save")}
            </Button>
          </Group>
        </Stack>
      </form>

      <Modal
        opened={confirmOpened}
        onClose={confirmModal.close}
        title={t("go_live")}
        centered
        radius="md"
      >
        <Stack gap="md">
          <Text size="sm">{t("go_live_confirm")}</Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={confirmModal.close}>
              {t("back")}
            </Button>
            <Button color="dark" onClick={goLive} loading={goingLive}>
              {t("go_live")}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </div>
  );
}
