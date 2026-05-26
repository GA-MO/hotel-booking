"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

import {
  ActionIcon,
  Anchor,
  Badge,
  Button,
  Card,
  Center,
  Checkbox,
  Chip,
  Group,
  Loader,
  Modal,
  NativeSelect,
  NumberInput,
  ScrollArea,
  SimpleGrid,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";

import { useShell } from "@/app/components/AppShell";
import {
  EmptyState,
  ErrorBanner,
  PageHeader,
} from "@/app/components/ui";
import { Pricing, RoomTypes } from "@/app/lib/api";
import { notifySuccess } from "@/app/lib/notify";
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
const MODIFIER_TYPES: PricingModifierType[] = [
  "percentage",
  "fixed_amount",
  "set_value",
];
const DAYS = [
  { v: 1, l: "Mon" },
  { v: 2, l: "Tue" },
  { v: 3, l: "Wed" },
  { v: 4, l: "Thu" },
  { v: 5, l: "Fri" },
  { v: 6, l: "Sat" },
  { v: 7, l: "Sun" },
];

const emptyRuleForm = (): CreatePricingRuleRequest => ({
  name: "",
  rule_type: "season",
  modifier_type: "percentage",
  modifier_value: "10",
  enabled: true,
});

export default function PricingPage() {
  const { activeHotel } = useShell();
  const [rules, setRules] = useState<PricingRule[]>([]);
  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modal, setModal] = useState<PricingRule | "new" | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<PricingRule | null>(null);
  const [deleting, setDeleting] = useState(false);

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

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await Pricing.remove(activeHotel.id, deleteTarget.id);
      notifySuccess(`${t("delete")} ✓`);
      setDeleteTarget(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setDeleting(false);
    }
  }

  return (
    <div>
      <PageHeader
        title={t("nav_pricing")}
        description={activeHotel.name}
        actions={
          <Button onClick={() => setModal("new")} color="dark">
            {t("add")}
          </Button>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      ) : rules.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <ScrollArea>
          <Table
            highlightOnHover
            withTableBorder
            verticalSpacing="sm"
            miw={700}
          >
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Name</Table.Th>
                <Table.Th>Type</Table.Th>
                <Table.Th>Modifier</Table.Th>
                <Table.Th>Range</Table.Th>
                <Table.Th>Priority</Table.Th>
                <Table.Th>Enabled</Table.Th>
                <Table.Th></Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rules.map((r) => (
                <Table.Tr key={r.id}>
                  <Table.Td>{r.name}</Table.Td>
                  <Table.Td>
                    <Text size="xs">{r.rule_type}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs" ff="monospace">
                      {r.modifier_type} / {r.modifier_value}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs">
                      {r.start_date || "—"} → {r.end_date || "—"}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs">{r.priority}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      size="sm"
                      color={r.enabled ? "teal" : "gray"}
                      variant="light"
                    >
                      {r.enabled ? "yes" : "no"}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Group gap={4} justify="flex-end">
                      <ActionIcon
                        variant="subtle"
                        color="gray"
                        onClick={() => setModal(r)}
                        aria-label={t("edit")}
                      >
                        ✎
                      </ActionIcon>
                      <ActionIcon
                        variant="subtle"
                        color="red"
                        onClick={() => setDeleteTarget(r)}
                        aria-label={t("delete")}
                      >
                        ✕
                      </ActionIcon>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      )}

      <Card withBorder radius="md" padding="lg" mt="xl">
        <Title order={3} size="h5" mb="sm">
          Availability overrides
        </Title>
        <Text size="sm" c="dimmed">
          Per-day overrides (close-outs, special rates, blocks) are edited on the
          calendar, where the cell grid mirrors the underlying data shape. Open
          the calendar and toggle{" "}
          <Text component="span" fw={500} c="dark">
            {t("bulk_edit")}
          </Text>{" "}
          to select cells and apply changes.
        </Text>
        <Anchor component={Link} href="/calendar" mt="xs" fw={500} c="dark">
          {t("nav_calendar")} →
        </Anchor>
      </Card>

      <RuleModal
        target={modal}
        onClose={() => setModal(null)}
        hotelID={activeHotel.id}
        roomTypes={roomTypes}
        onSaved={async () => {
          setModal(null);
          await load();
        }}
      />

      <Modal
        opened={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t("delete")}
        centered
        radius="md"
      >
        <Stack gap="md">
          <Text size="sm">
            {t("delete")} <b>{deleteTarget?.name}</b>?
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setDeleteTarget(null)}>
              {t("cancel")}
            </Button>
            <Button color="red" loading={deleting} onClick={confirmDelete}>
              {t("delete")}
            </Button>
          </Group>
        </Stack>
      </Modal>
    </div>
  );
}

function RuleModal({
  target,
  onClose,
  hotelID,
  roomTypes,
  onSaved,
}: {
  target: PricingRule | "new" | null;
  onClose: () => void;
  hotelID: string;
  roomTypes: RoomType[];
  onSaved: () => Promise<void>;
}) {
  const rule = target && target !== "new" ? target : null;
  const opened = target !== null;

  const [form, setForm] = useState<CreatePricingRuleRequest>(emptyRuleForm());
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!opened) return;
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
      setForm(emptyRuleForm());
    }
    setErr(null);
  }, [opened, rule]);

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
        notifySuccess(`${t("save")} ✓`);
      } else {
        await Pricing.create(hotelID, form);
        notifySuccess(`${t("add")} ✓`);
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
    const next = arr.includes(day)
      ? arr.filter((d) => d !== day)
      : [...arr, day].sort();
    setForm({ ...form, days_of_week: next });
  }

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={rule ? t("edit") : t("add")}
      centered
      radius="md"
      size="lg"
    >
      <form onSubmit={submit}>
        <Stack gap="sm">
          <TextInput
            label="Name"
            required
            value={form.name}
            onChange={(e) => setForm({ ...form, name: e.currentTarget.value })}
          />
          <SimpleGrid cols={2} spacing="sm">
            <NativeSelect
              label="Rule type"
              data={RULE_TYPES}
              value={form.rule_type}
              onChange={(e) =>
                setForm({
                  ...form,
                  rule_type: e.currentTarget.value as PricingRuleType,
                })
              }
            />
            <NativeSelect
              label="Room type (optional)"
              data={[
                { value: "", label: "All" },
                ...roomTypes.map((r) => ({ value: r.id, label: r.name })),
              ]}
              value={form.room_type_id || ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  room_type_id: e.currentTarget.value || null,
                })
              }
              disabled={!!rule}
            />
            <NativeSelect
              label="Modifier type"
              data={MODIFIER_TYPES}
              value={form.modifier_type}
              onChange={(e) =>
                setForm({
                  ...form,
                  modifier_type: e.currentTarget.value as PricingModifierType,
                })
              }
            />
            <TextInput
              label="Modifier value"
              required
              value={form.modifier_value}
              onChange={(e) =>
                setForm({ ...form, modifier_value: e.currentTarget.value })
              }
            />
            <TextInput
              label="Start date"
              type="date"
              value={form.start_date || ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  start_date: e.currentTarget.value || null,
                })
              }
            />
            <TextInput
              label="End date"
              type="date"
              value={form.end_date || ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  end_date: e.currentTarget.value || null,
                })
              }
            />
            <NumberInput
              label="Min nights"
              min={1}
              value={form.min_nights ?? ""}
              onChange={(v) =>
                setForm({
                  ...form,
                  min_nights: typeof v === "number" ? v : null,
                })
              }
            />
            <NumberInput
              label="Priority"
              value={form.priority ?? 0}
              onChange={(v) =>
                setForm({
                  ...form,
                  priority: typeof v === "number" ? v : 0,
                })
              }
            />
          </SimpleGrid>

          <Stack gap={4}>
            <Text size="sm" fw={500} c="gray.7">
              Days of week (Mon=1, Sun=7)
            </Text>
            <Chip.Group multiple value={(form.days_of_week || []).map(String)}>
              <Group gap="xs">
                {DAYS.map((d) => (
                  <Chip
                    key={d.v}
                    value={String(d.v)}
                    onClick={() => toggleDay(d.v)}
                    color="dark"
                  >
                    {d.l}
                  </Chip>
                ))}
              </Group>
            </Chip.Group>
          </Stack>

          <Checkbox
            label="Enabled"
            checked={form.enabled ?? true}
            onChange={(e) =>
              setForm({ ...form, enabled: e.currentTarget.checked })
            }
          />

          <ErrorBanner message={err} />
          <Group justify="flex-end" mt="sm">
            <Button type="button" variant="default" onClick={onClose}>
              {t("cancel")}
            </Button>
            <Button type="submit" loading={saving} color="dark">
              {t("save")}
            </Button>
          </Group>
        </Stack>
      </form>
    </Modal>
  );
}
