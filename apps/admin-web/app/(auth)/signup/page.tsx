"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import {
  Anchor,
  Button,
  Card,
  Group,
  NativeSelect,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";

import { Auth, ApiClientError } from "@/app/lib/api";
import { setTokens } from "@/app/lib/auth";
import { ErrorBanner } from "@/app/components/ui";
import { t } from "@/app/i18n";

const COUNTRIES = [
  { value: "TH", label: "TH — Thailand" },
  { value: "SG", label: "SG — Singapore" },
  { value: "MY", label: "MY — Malaysia" },
  { value: "VN", label: "VN — Vietnam" },
  { value: "ID", label: "ID — Indonesia" },
  { value: "PH", label: "PH — Philippines" },
];

const LOCALES = [
  { value: "th", label: "ไทย" },
  { value: "en", label: "English" },
];

export default function SignupPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const form = useForm({
    initialValues: {
      name: "",
      email: "",
      password: "",
      country: "TH",
      locale: "th",
    },
    validate: {
      name: (v) => (v.trim() ? null : t("required_field")),
      email: (v) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? null : t("invalid_email"),
      password: (v) =>
        v.length >= 8 ? null : "Password must be at least 8 characters",
    },
  });

  async function onSubmit(values: typeof form.values) {
    setError(null);
    setLoading(true);
    try {
      const res = await Auth.signup(values);
      setTokens({
        accessToken: res.access_token,
        refreshToken: res.refresh_token,
        user: res.user,
      });
      router.replace("/onboarding");
    } catch (err) {
      const msg = err instanceof ApiClientError ? err.message : t("error_generic");
      setError(msg);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Card withBorder radius="md" shadow="sm" padding="xl">
      <Title order={1} size="h3">
        {t("signup_title")}
      </Title>
      <Text size="sm" c="dimmed" mt={4}>
        {t("signup_subtitle")}
      </Text>

      <form onSubmit={form.onSubmit(onSubmit)}>
        <Stack mt="lg" gap="md">
          <TextInput
            label={t("name")}
            autoComplete="name"
            required
            {...form.getInputProps("name")}
          />
          <TextInput
            label={t("email")}
            type="email"
            autoComplete="email"
            required
            {...form.getInputProps("email")}
          />
          <PasswordInput
            label={t("password")}
            autoComplete="new-password"
            required
            {...form.getInputProps("password")}
          />
          <Group grow>
            <NativeSelect
              label={t("country")}
              data={COUNTRIES}
              {...form.getInputProps("country")}
            />
            <NativeSelect
              label={t("locale")}
              data={LOCALES}
              {...form.getInputProps("locale")}
            />
          </Group>
          <ErrorBanner message={error} />
          <Button type="submit" loading={loading} fullWidth color="dark">
            {t("sign_up")}
          </Button>
        </Stack>
      </form>

      <Text size="sm" c="dimmed" mt="lg">
        {t("have_account")}{" "}
        <Anchor component={Link} href="/login" fw={500} c="dark">
          {t("log_in")}
        </Anchor>
      </Text>
    </Card>
  );
}
