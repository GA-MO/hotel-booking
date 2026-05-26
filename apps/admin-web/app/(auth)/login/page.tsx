"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { Auth, ApiClientError } from "@/app/lib/api";
import { setTokens } from "@/app/lib/auth";
import { Button, ErrorBanner, Field, TextInput } from "@/app/components/ui";
import { t } from "@/app/i18n";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await Auth.login({ email, password });
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
    <div className="rounded-lg border border-neutral-200 bg-white p-6 shadow-sm">
      <h1 className="text-xl font-semibold">{t("login_title")}</h1>
      <p className="mt-1 text-sm text-neutral-600">{t("login_subtitle")}</p>

      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <Field label={t("email")}>
          <TextInput
            type="email"
            required
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </Field>
        <Field label={t("password")}>
          <TextInput
            type="password"
            required
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>
        <ErrorBanner message={error} />
        <Button type="submit" loading={loading} className="w-full">
          {t("log_in")}
        </Button>
      </form>

      <p className="mt-6 text-sm text-neutral-600">
        {t("no_account")}{" "}
        <Link href="/signup" className="font-medium text-neutral-900 underline">
          {t("sign_up")}
        </Link>
      </p>
    </div>
  );
}
