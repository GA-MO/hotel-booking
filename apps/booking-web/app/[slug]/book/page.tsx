import { headers } from "next/headers";
import { notFound } from "next/navigation";

import LanguageSwitcher from "../../components/LanguageSwitcher";
import { DEFAULT_LOCALE, getDict, resolveLocale, type Locale } from "../../i18n";
import { ApiError, getLanding } from "../../lib/api";
import type { LandingPage } from "../../lib/types";

import BookForm from "./BookForm";

type Params = { slug: string };
type SearchParams = { lang?: string | string[]; room_type_id?: string };

export const dynamic = "force-dynamic";

async function load(slug: string, locale: Locale): Promise<LandingPage | null> {
  try {
    return await getLanding(slug, locale);
  } catch (err) {
    if (err instanceof ApiError && err.isNotFound()) {
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

// extractRooms pulls room candidates out of the landing page's `rooms`
// section content. The booking-web app has no direct access to the
// room_types table; we depend on whatever the admin published.
function extractRooms(page: LandingPage) {
  const section = page.sections.find((s) => s.type === "rooms" && s.enabled);
  if (!section) return [];
  // Content is opaque to the API; cast to the FE-side type.
  const c = section.content as { rooms?: Array<{
    id: string;
    name: string;
    base_rate?: string;
    base_currency?: string;
    max_occupancy?: number;
  }> };
  return c.rooms ?? [];
}

export default async function BookPage({
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

  const rooms = extractRooms(page);

  return (
    <main className="font-sans">
      <header className="mx-auto flex max-w-3xl items-center justify-between px-6 py-4">
        <a href={`/${slug}?lang=${locale}`} className="text-sm text-neutral-600 hover:underline">
          ← {page.seo.title || slug}
        </a>
        <LanguageSwitcher current={locale} pathname={`/${slug}/book`} />
      </header>

      <section className="mx-auto max-w-3xl px-6 pb-16 pt-4">
        <h1 className="mb-6 text-3xl font-semibold tracking-tight">{dict.book.title}</h1>
        {rooms.length === 0 ? (
          <p className="rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-amber-900">
            {dict.book.no_rooms}
          </p>
        ) : (
          <BookForm
            slug={slug}
            locale={locale}
            dict={dict}
            rooms={rooms}
            initialRoomId={sp.room_type_id}
          />
        )}
      </section>
    </main>
  );
}
