"use client";

// Thin wrappers over Mantine v9 primitives. The export surface mirrors what
// the legacy pages already import (Button, TextInput, Field, Card, Modal,
// StatusBadge, etc.) so the page-level migration can happen incrementally
// without breaking the build.
//
// For new code, prefer importing directly from "@mantine/core" /
// "@mantine/notifications" rather than this shim.

import type { ReactNode } from "react";

import {
  Alert,
  Badge,
  Button as MantineButton,
  Card as MantineCard,
  Group,
  Modal as MantineModal,
  NativeSelect as MantineNativeSelect,
  Stack,
  Text,
  TextInput as MantineTextInput,
  Textarea as MantineTextarea,
  Title,
  type ButtonProps as MantineButtonProps,
  type ModalProps as MantineModalProps,
  type NativeSelectProps as MantineNativeSelectProps,
  type TextInputProps as MantineTextInputProps,
  type TextareaProps as MantineTextareaProps,
} from "@mantine/core";

type Variant = "primary" | "secondary" | "danger" | "ghost";

const variantToMantine: Record<
  Variant,
  { variant: MantineButtonProps["variant"]; color?: string }
> = {
  primary: { variant: "filled", color: "dark" },
  secondary: { variant: "default" },
  danger: { variant: "filled", color: "red" },
  ghost: { variant: "subtle", color: "gray" },
};

export function Button({
  variant = "primary",
  loading,
  children,
  type,
  onClick,
  disabled,
  className,
  ...rest
}: {
  variant?: Variant;
  loading?: boolean;
  children: ReactNode;
  type?: "button" | "submit" | "reset";
  onClick?: React.MouseEventHandler<HTMLButtonElement>;
  disabled?: boolean;
  className?: string;
} & Omit<MantineButtonProps, "variant" | "color" | "loading" | "children">) {
  const v = variantToMantine[variant];
  return (
    <MantineButton
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={className}
      loading={loading}
      variant={v.variant}
      color={v.color}
      size="sm"
      {...rest}
    >
      {children}
    </MantineButton>
  );
}

export function Field({
  label,
  hint,
  error,
  children,
}: {
  label?: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  // Mantine inputs carry their own label/error/description. This wrapper is
  // for the cases where pages still mount a raw <input> inside a Field —
  // we render a plain block label+hint/error stack.
  return (
    <Stack gap={4}>
      {label && (
        <Text size="sm" fw={500} c="gray.7">
          {label}
        </Text>
      )}
      {children}
      {hint && !error && (
        <Text size="xs" c="dimmed">
          {hint}
        </Text>
      )}
      {error && (
        <Text size="xs" c="red">
          {error}
        </Text>
      )}
    </Stack>
  );
}

export function TextInput(props: MantineTextInputProps) {
  return <MantineTextInput size="sm" {...props} />;
}

export function TextArea(props: MantineTextareaProps) {
  return <MantineTextarea size="sm" autosize minRows={3} {...props} />;
}

// Use NativeSelect (renders a real <select>) instead of Mantine's combobox
// Select. This keeps the legacy `onChange={(e) => setX(e.target.value)}`
// pattern working everywhere in admin-web. For new code that needs search,
// keyboard nav, or grouped options, import { Select } from "@mantine/core"
// directly and use its (value, option) signature.
export function Select(props: MantineNativeSelectProps) {
  return <MantineNativeSelect size="sm" {...props} />;
}

export function Card({
  title,
  actions,
  children,
  className,
}: {
  title?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <MantineCard
      withBorder
      shadow="xs"
      padding="lg"
      radius="md"
      className={className}
    >
      {(title || actions) && (
        <Group justify="space-between" align="flex-start" mb="md" wrap="nowrap">
          {title && <Title order={3} size="h5">{title}</Title>}
          {actions && <Group gap="xs">{actions}</Group>}
        </Group>
      )}
      {children}
    </MantineCard>
  );
}

// Status palette — keep the same value→tone map as before so existing pages
// look identical. Mantine Badge colors map to its semantic palette.
const statusToColor: Record<string, string> = {
  pending_payment: "yellow",
  confirmed: "teal",
  cancelled: "red",
  expired: "gray",
  checked_in: "blue",
  checked_out: "gray",
  no_show: "red",
  completed: "teal",
  draft: "gray",
  published: "teal",
  active: "teal",
  trialing: "blue",
  trial_ending: "yellow",
  trial_lapsed: "red",
  past_due: "yellow",
  suspended: "red",
  test: "yellow",
  live: "teal",
};

export function StatusBadge({ value }: { value: string }) {
  const color = statusToColor[value] || "gray";
  return (
    <Badge variant="light" color={color} size="sm" radius="sm">
      {value.replace(/_/g, " ")}
    </Badge>
  );
}

export function ErrorBanner({ message }: { message?: string | null }) {
  if (!message) return null;
  return (
    <Alert color="red" radius="md" variant="light">
      {message}
    </Alert>
  );
}

export function SuccessBanner({ message }: { message?: string | null }) {
  if (!message) return null;
  return (
    <Alert color="teal" radius="md" variant="light">
      {message}
    </Alert>
  );
}

export function EmptyState({ message }: { message: string }) {
  return (
    <Card>
      <Text ta="center" c="dimmed" size="sm" py="xl">
        {message}
      </Text>
    </Card>
  );
}

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <Group justify="space-between" align="flex-end" mb="lg" wrap="wrap">
      <div>
        <Title order={1} size="h3">
          {title}
        </Title>
        {description && (
          <Text size="sm" c="dimmed" mt={4}>
            {description}
          </Text>
        )}
      </div>
      {actions && <Group gap="xs">{actions}</Group>}
    </Group>
  );
}

export function Modal({
  open,
  onClose,
  title,
  children,
  size,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  size?: MantineModalProps["size"];
}) {
  // Mantine handles focus trap, portal, ESC-to-close, scroll lock, ARIA roles.
  return (
    <MantineModal
      opened={open}
      onClose={onClose}
      title={title}
      centered
      radius="md"
      size={size || "lg"}
      overlayProps={{ backgroundOpacity: 0.5, blur: 2 }}
    >
      {children}
    </MantineModal>
  );
}
