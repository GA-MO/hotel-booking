"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { Auth, ApiClientError } from "@/app/lib/api";
import { setTokens } from "@/app/lib/auth";
import { Button, ErrorBanner, Field, Select, TextInput } from "@/app/components/ui";
import { t } from "@/app/i18n";

export default function SignupPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [country, setCountry] = useState("TH");
  const [locale, setLocale] = useState("th");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await Auth.signup({ email, password, name, locale, country });
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
    <div className="rounded-lg border border-neutral-200 bg-white p-6 shadow-sm">
      <h1 className="text-xl font-semibold">{t("signup_title")}</h1>
      <p className="mt-1 text-sm text-neutral-600">{t("signup_subtitle")}</p>

      <form onSubmit={onSubmit} className="mt-6 space-y-4">
        <Field label={t("name")}>
          <TextInput
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoComplete="name"
          />
        </Field>
        <Field label={t("email")}>
          <TextInput
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
          />
        </Field>
        <Field label={t("password")}>
          <TextInput
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
          />
        </Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label={t("country")}>
            <Select value={country} onChange={(e) => setCountry(e.target.value)}>
              <option value="TH">TH — Thailand</option>
              <option value="SG">SG — Singapore</option>
              <option value="MY">MY — Malaysia</option>
              <option value="VN">VN — Vietnam</option>
              <option value="ID">ID — Indonesia</option>
              <option value="PH">PH — Philippines</option>
            </Select>
          </Field>
          <Field label={t("locale")}>
            <Select value={locale} onChange={(e) => setLocale(e.target.value)}>
              <option value="th">ไทย</option>
              <option value="en">English</option>
            </Select>
          </Field>
        </div>
        <ErrorBanner message={error} />
        <Button type="submit" loading={loading} className="w-full">
          {t("sign_up")}
        </Button>
      </form>

      <p className="mt-6 text-sm text-neutral-600">
        {t("have_account")}{" "}
        <Link href="/login" className="font-medium text-neutral-900 underline">
          {t("log_in")}
        </Link>
      </p>
    </div>
  );
}
