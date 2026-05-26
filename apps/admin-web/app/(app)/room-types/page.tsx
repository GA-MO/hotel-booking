"use client";

import { useEffect, useState } from "react";

import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Center,
  Group,
  Loader,
  Modal,
  NumberInput,
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
import PhotoUploader from "@/app/components/PhotoUploader";
import {
  EmptyState,
  ErrorBanner,
  PageHeader,
} from "@/app/components/ui";
import { Photos, RoomTypes } from "@/app/lib/api";
import { notifySuccess } from "@/app/lib/notify";
import { t } from "@/app/i18n";
import type { Photo, RoomType, RoomTypeCreateRequest } from "@/app/lib/types";

type FormValues = Required<
  Pick<
    RoomTypeCreateRequest,
    "name" | "description" | "total_inventory" | "max_occupancy" | "base_rate" | "base_currency"
  >
> & { display_order?: number };

const emptyValues = (defaultCurrency: string): FormValues => ({
  name: "",
  description: "",
  total_inventory: 1,
  max_occupancy: 2,
  base_rate: 1500,
  base_currency: defaultCurrency,
});

export default function RoomTypesPage() {
  const { activeHotel } = useShell();
  const [items, setItems] = useState<RoomType[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalState, setModalState] = useState<
    { mode: "create" } | { mode: "edit"; rt: RoomType } | null
  >(null);
  const [deleteTarget, setDeleteTarget] = useState<RoomType | null>(null);
  const [deleting, setDeleting] = useState(false);

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

  async function confirmDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await RoomTypes.remove(activeHotel.id, deleteTarget.id);
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
        title={t("nav_room_types")}
        description={activeHotel.name}
        actions={
          <Button onClick={() => setModalState({ mode: "create" })} color="dark">
            {t("add")}
          </Button>
        }
      />

      <ErrorBanner message={error} />

      {loading ? (
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      ) : items.length === 0 ? (
        <EmptyState message={t("no_data")} />
      ) : (
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3 }} spacing="md">
          {items.map((rt) => (
            <Card key={rt.id} withBorder radius="md" padding="lg">
              <Group justify="space-between" align="flex-start" wrap="nowrap" mb="sm">
                <Title order={4} size="h5" lineClamp={2}>
                  {rt.name}
                </Title>
                <Group gap={4} wrap="nowrap">
                  <ActionIcon
                    variant="subtle"
                    color="gray"
                    onClick={() => setModalState({ mode: "edit", rt })}
                    aria-label={t("edit")}
                  >
                    ✎
                  </ActionIcon>
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    onClick={() => setDeleteTarget(rt)}
                    aria-label={t("delete")}
                  >
                    ✕
                  </ActionIcon>
                </Group>
              </Group>

              <Stack gap={6}>
                <Row label={t("total_inventory")} value={rt.total_inventory} />
                <Row label={t("max_occupancy")} value={rt.max_occupancy} />
                <Row
                  label={t("base_rate")}
                  value={`${rt.base_rate.toFixed(2)} ${rt.base_currency}`}
                  mono
                />
                <Row
                  label="Enabled"
                  value={
                    <Badge
                      size="sm"
                      color={rt.enabled ? "teal" : "gray"}
                      variant="light"
                    >
                      {rt.enabled ? "yes" : "no"}
                    </Badge>
                  }
                />
              </Stack>

              {rt.description && (
                <Text size="xs" c="dimmed" mt="sm">
                  {rt.description}
                </Text>
              )}
            </Card>
          ))}
        </SimpleGrid>
      )}

      <RoomTypeFormModal
        state={modalState}
        onClose={() => setModalState(null)}
        hotelID={activeHotel.id}
        defaultCurrency={activeHotel.base_currency}
        onSaved={async () => {
          setModalState(null);
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

function Row({
  label,
  value,
  mono,
}: {
  label: string;
  value: React.ReactNode;
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

function RoomTypeFormModal({
  state,
  onClose,
  hotelID,
  defaultCurrency,
  onSaved,
}: {
  state: { mode: "create" } | { mode: "edit"; rt: RoomType } | null;
  onClose: () => void;
  hotelID: string;
  defaultCurrency: string;
  onSaved: () => Promise<void>;
}) {
  const editing = state?.mode === "edit";
  const rt = editing ? state.rt : null;

  const [opened, modal] = useDisclosure(false);
  const [photos, setPhotos] = useState<Photo[]>([]);
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const form = useForm<FormValues>({
    initialValues: emptyValues(defaultCurrency),
    validate: {
      name: (v) => (v.trim() ? null : t("required_field")),
    },
  });

  useEffect(() => {
    if (state === null) {
      modal.close();
      return;
    }
    modal.open();
    if (rt) {
      form.setValues({
        name: rt.name,
        description: rt.description || "",
        total_inventory: rt.total_inventory,
        max_occupancy: rt.max_occupancy,
        base_rate: rt.base_rate,
        base_currency: rt.base_currency,
        display_order: rt.display_order,
      });
      Photos.listRoomType(hotelID, rt.id)
        .then((r) => setPhotos(r.photos))
        .catch(() => setPhotos([]));
    } else {
      form.setValues(emptyValues(defaultCurrency));
      setPhotos([]);
    }
    setErr(null);
    // form is intentionally not in deps — its setValues would loop the effect
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [state, defaultCurrency, hotelID]);

  async function submit(values: FormValues) {
    setSaving(true);
    setErr(null);
    try {
      if (rt) {
        await RoomTypes.update(hotelID, rt.id, values);
        notifySuccess(`${t("save")} ✓`);
      } else {
        await RoomTypes.create(hotelID, values);
        notifySuccess(`${t("add")} ✓`);
      }
      await onSaved();
    } catch (e) {
      setErr(e instanceof Error ? e.message : t("error_generic"));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={rt ? t("edit") : t("add")}
      centered
      radius="md"
      size="lg"
    >
      <form onSubmit={form.onSubmit(submit)}>
        <Stack gap="sm">
          <TextInput
            label={t("room_type_name")}
            required
            {...form.getInputProps("name")}
          />
          <Textarea
            label={t("description")}
            rows={2}
            {...form.getInputProps("description")}
          />
          <SimpleGrid cols={2} spacing="sm">
            <NumberInput
              label={t("total_inventory")}
              min={0}
              required
              {...form.getInputProps("total_inventory")}
            />
            <NumberInput
              label={t("max_occupancy")}
              min={1}
              required
              {...form.getInputProps("max_occupancy")}
            />
            <NumberInput
              label={t("base_rate")}
              min={0}
              step={0.01}
              decimalScale={2}
              required
              {...form.getInputProps("base_rate")}
            />
            <TextInput
              label={t("currency")}
              {...form.getInputProps("base_currency")}
            />
          </SimpleGrid>
          {rt && (
            <Stack gap={4}>
              <Text size="sm" fw={500} c="gray.7">
                {t("upload_photo")}
              </Text>
              <PhotoUploader
                hotelID={hotelID}
                kind="room_type_photo"
                roomTypeID={rt.id}
                photos={photos}
                onChange={setPhotos}
              />
            </Stack>
          )}
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
