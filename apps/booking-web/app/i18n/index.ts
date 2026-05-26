import en from "./en";
import th from "./th";

export type Locale = "th" | "en";

export type Dict = {
  common: {
    book_now: string;
    loading: string;
    error_generic: string;
    error_not_found: string;
    language_label: string;
    night: string;
    nights: string;
    room: string;
    rooms: string;
    guest: string;
    guests: string;
  };
  landing: {
    skip_to_book: string;
    no_hero_image: string;
    gallery_title: string;
    rooms_title: string;
    amenities_title: string;
    location_title: string;
    reviews_title: string;
    rating_label: string;
    from_price: string;
    occupancy: string;
    select_this_room: string;
  };
  book: {
    title: string;
    check_in: string;
    check_out: string;
    rooms: string;
    room_type: string;
    guest_details: string;
    name: string;
    email: string;
    phone: string;
    special_requests: string;
    submit: string;
    submitting: string;
    quote_loading: string;
    quote_subtotal: string;
    quote_total: string;
    quote_per_night: string;
    quote_select_dates: string;
    quote_unavailable: string;
    no_rooms: string;
    summary_title: string;
    pick_room_first: string;
    invalid_dates: string;
  };
  confirmation: {
    title: string;
    reference: string;
    status: string;
    summary: string;
    guest: string;
    dates: string;
    total: string;
    payment_title: string;
    payment_hint: string;
    payment_paid_button: string;
    payment_paid_thanks: string;
    cancel_title: string;
    cancel_button: string;
    cancel_confirm: string;
    cancel_reason_label: string;
    cancel_success: string;
    look_up_title: string;
    look_up_hint: string;
    look_up_button: string;
  };
  status: Record<
    | "pending_payment"
    | "confirmed"
    | "cancelled"
    | "expired"
    | "checked_in"
    | "checked_out"
    | "no_show"
    | "completed",
    string
  >;
  payment_status: Record<
    "pending" | "paid" | "refunded" | "partial" | "failed",
    string
  >;
};

const DICTS: Record<Locale, Dict> = { en, th };

export const DEFAULT_LOCALE: Locale = "th";
export const SUPPORTED_LOCALES: ReadonlyArray<Locale> = ["th", "en"];

export function isLocale(s: unknown): s is Locale {
  return typeof s === "string" && (s === "th" || s === "en");
}

export function getDict(locale: Locale): Dict {
  return DICTS[locale];
}

// resolveLocale picks the active locale for a request given a `?lang=` query
// param and an Accept-Language header. The query param wins; otherwise we
// fall back to the first supported locale in the header; otherwise default.
export function resolveLocale(
  langParam: string | string[] | undefined,
  acceptLanguage: string | null,
): Locale {
  const raw = Array.isArray(langParam) ? langParam[0] : langParam;
  if (isLocale(raw)) return raw;

  if (acceptLanguage) {
    const langs = acceptLanguage
      .split(",")
      .map((p) => p.split(";")[0].trim().toLowerCase().slice(0, 2));
    for (const l of langs) {
      if (isLocale(l)) return l;
    }
  }
  return DEFAULT_LOCALE;
}
