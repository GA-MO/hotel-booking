"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import {
  Anchor,
  Button,
  Card,
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

export default function LoginPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const form = useForm({
    initialValues: { email: "", password: "" },
    validate: {
      email: (v) =>
        /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? null : t("invalid_email"),
      password: (v) => (v.length >= 1 ? null : t("required_field")),
    },
  });

  async function onSubmit(values: typeof form.values) {
    setError(null);
    setLoading(true);
    try {
      const res = await Auth.login(values);
      setTokens({
        accessToken: res.access_token,
        refreshToken: res.refresh_token,
        user: res.user,
      });
      router.replace("/");
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
        {t("login_title")}
      </Title>
      <Text size="sm" c="dimmed" mt={4}>
        {t("login_subtitle")}
      </Text>

      <form onSubmit={form.onSubmit(onSubmit)}>
        <Stack mt="lg" gap="md">
          <TextInput
            label={t("email")}
            type="email"
            autoComplete="email"
            required
            {...form.getInputProps("email")}
          />
          <PasswordInput
            label={t("password")}
            autoComplete="current-password"
            required
            {...form.getInputProps("password")}
          />
          <ErrorBanner message={error} />
          <Button type="submit" loading={loading} fullWidth color="dark">
            {t("log_in")}
          </Button>
        </Stack>
      </form>

      <Text size="sm" c="dimmed" mt="lg">
        {t("no_account")}{" "}
        <Anchor component={Link} href="/signup" fw={500} c="dark">
          {t("sign_up")}
        </Anchor>
      </Text>
    </Card>
  );
}
