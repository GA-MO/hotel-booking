import type { Metadata } from "next";
import { headers } from "next/headers";
import { notFound } from "next/navigation";

import LanguageSwitcher from "../components/LanguageSwitcher";
import SectionRenderer from "../components/SectionRenderer";
import TrackingScripts from "../components/TrackingScripts";
import { DEFAULT_LOCALE, getDict, resolveLocale, type Locale } from "../i18n";
import { ApiError, getLanding } from "../lib/api";
import type { LandingPage } from "../lib/types";

type Params = { slug: string };
type SearchParams = { lang?: string | string[] };

// ISR — re-render at most once a minute. Underlying API call is also cached
// with revalidate=3600 as a backstop (see lib/api.ts).
export const revalidate = 60;

async function load(slug: string, locale: Locale): Promise<LandingPage | null> {
  try {
    return await getLanding(slug, locale);
  } catch (err) {
    if (err instanceof ApiError && err.isNotFound()) {
      // Fallback to default locale before giving up — common when a hotel
      // has only published TH content but the visitor's browser asks for EN.
      if (locale !== DEFAULT_LOCALE) {
        try {
          return await getLanding(slug, DEFAULT_LOCALE);
        } catch {
          return null;
        }
      }
      return null;
    }
    throw err;
  }
}

export async function generateMetadata({
  params,
  searchParams,
}: {
  params: Promise<Params>;
  searchParams: Promise<SearchParams>;
}): Promise<Metadata> {
  const { slug } = await params;
  const sp = await searchParams;
  const hdrs = await headers();
  const locale = resolveLocale(sp.lang, hdrs.get("accept-language"));

  const page = await load(slug, locale);
  if (!page) {
    return { title: "Not found" };
  }

  const title = page.seo.title || slug;
  const description = page.seo.description || undefined;
  const ogImage = page.seo.og_image_url;

  return {
    title,
    description,
    openGraph: {
      title,
      description,
      images: ogImage ? [ogImage] : undefined,
      type: "website",
      locale: page.locale,
    },
    twitter: {
      card: ogImage ? "summary_large_image" : "summary",
      title,
      description,
      images: ogImage ? [ogImage] : undefined,
    },
  };
}

function brandingVars(p: LandingPage): React.CSSProperties {
  // Branding is applied via CSS custom properties on a wrapper so component
  // styles can read --brand-primary / --brand-accent / --brand-font without
  // each component knowing about the branding object.
  const out: Record<string, string> = {};
  if (p.branding.primary_color) out["--brand-primary"] = p.branding.primary_color;
  if (p.branding.accent_color) out["--brand-accent"] = p.branding.accent_color;
  if (p.branding.font_family) out["--brand-font"] = p.branding.font_family;
  return out as React.CSSProperties;
}

export default async function HotelLandingPage({
  params,
  searchParams,
}: {
  params: Promise<Params>;
  searchParams: Promise<SearchParams>;
}) {
  const { slug } = await params;
  const sp = await searchParams;
  const hdrs = await headers();
  const locale = resolveLocale(sp.lang, hdrs.get("accept-language"));
  const dict = getDict(locale);

  const page = await load(slug, locale);
  if (!page) notFound();

  return (
    <main
      style={brandingVars(page)}
      className="font-sans"
    >
      <TrackingScripts tracking={page.tracking} />

      <header className="mx-auto flex max-w-5xl items-center justify-between px-6 py-4">
        <div className="flex items-center gap-3">
          {page.branding.logo_url ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={page.branding.logo_url}
              alt=""
              className="h-10 w-auto"
            />
          ) : (
            <span className="text-base font-semibold">{page.seo.title || slug}</span>
          )}
        </div>
        <LanguageSwitcher current={locale} pathname={`/${slug}`} />
      </header>

      <SectionRenderer sections={page.sections} slug={slug} dict={dict} locale={locale} />

      <footer className="mx-auto max-w-5xl px-6 py-8 text-sm text-neutral-500">
        <a href={`/${slug}/book?lang=${locale}`} className="underline">
          {dict.common.book_now}
        </a>
      </footer>
    </main>
  );
}
