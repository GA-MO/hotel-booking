import { headers } from "next/headers";

import LanguageSwitcher from "../../../components/LanguageSwitcher";
import { getDict, resolveLocale } from "../../../i18n";
import { ApiError, getBooking } from "../../../lib/api";
import { formatRange } from "../../../lib/dates";
import { formatMoney, localeFor } from "../../../lib/money";
import type { PublicBookingResponse } from "../../../lib/types";

import BookingActions from "./BookingActions";
import LookupForm from "./LookupForm";

type Params = { slug: string; reference: string };
type SearchParams = {
  lang?: string | string[];
  email?: string;
};

export const dynamic = "force-dynamic";

async function tryLoadBooking(reference: string, email: string | undefined) {
  if (!email) return { booking: null, error: null as ApiError | null };
  try {
    const booking = await getBooking(reference, email);
    return { booking, error: null };
  } catch (err) {
    if (err instanceof ApiError) {
      return { booking: null, error: err };
    }
    throw err;
  }
}

function BookingSummary({
  booking,
  locale,
  dict,
}: {
  booking: PublicBookingResponse;
  locale: "th" | "en";
  dict: ReturnType<typeof getDict>;
}) {
  const timezone = booking.hotel.timezone;
  return (
    <dl className="space-y-3 text-sm">
      <div className="flex justify-between">
        <dt className="text-neutral-500">{dict.confirmation.reference}</dt>
        <dd className="font-mono font-medium">{booking.reference}</dd>
      </div>
      <div className="flex justify-between">
        <dt className="text-neutral-500">{dict.confirmation.status}</dt>
        <dd className="font-medium">{dict.status[booking.status]}</dd>
      </div>
      <div className="flex justify-between">
        <dt className="text-neutral-500">{dict.confirmation.guest}</dt>
        <dd>{booking.guest_name}</dd>
      </div>
      <div className="flex justify-between">
        <dt className="text-neutral-500">{dict.confirmation.dates}</dt>
        <dd>{formatRange(booking.check_in_date, booking.check_out_date, { locale, timezone })}</dd>
      </div>
      <div className="flex justify-between border-t border-neutral-200 pt-3 text-base font-semibold">
        <dt>{dict.confirmation.total}</dt>
        <dd>{formatMoney(booking.total_cents, booking.currency, localeFor(locale))}</dd>
      </div>
    </dl>
  );
}

export default async function ConfirmationPage({
  params,
  searchParams,
}: {
  params: Promise<Params>;
  searchParams: Promise<SearchParams>;
}) {
  const { slug, reference } = await params;
  const sp = await searchParams;
  const hdrs = await headers();
  const locale = resolveLocale(sp.lang, hdrs.get("accept-language"));
  const dict = getDict(locale);

  const email = Array.isArray(sp.email) ? sp.email[0] : sp.email;
  const { booking, error } = await tryLoadBooking(reference, email);

  return (
    <main className="font-sans">
      <header className="mx-auto flex max-w-2xl items-center justify-between px-6 py-4">
        <a href={`/${slug}?lang=${locale}`} className="text-sm text-neutral-600 hover:underline">
          ←
        </a>
        <LanguageSwitcher
          current={locale}
          pathname={`/${slug}/booking/${reference}`}
        />
      </header>

      <section className="mx-auto max-w-2xl px-6 pb-16 pt-4">
        {booking ? (
          <>
            <h1 className="mb-6 text-3xl font-semibold tracking-tight">
              {dict.confirmation.title}
            </h1>

            <div className="space-y-6 rounded-xl border border-neutral-200 bg-white p-6">
              <BookingSummary booking={booking} locale={locale} dict={dict} />
            </div>

            <BookingActions booking={booking} locale={locale} dict={dict} slug={slug} />
          </>
        ) : (
          <>
            <h1 className="mb-3 text-2xl font-semibold">{dict.confirmation.look_up_title}</h1>
            <p className="mb-6 text-sm text-neutral-600">
              {dict.confirmation.look_up_hint}
            </p>
            {error && error.status !== 404 && (
              <p className="mb-4 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
                {dict.common.error_generic}
              </p>
            )}
            <LookupForm
              reference={reference}
              initialEmail={email ?? ""}
              dict={dict}
              locale={locale}
              slug={slug}
            />
          </>
        )}
      </section>
    </main>
  );
}
