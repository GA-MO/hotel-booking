// API DTO types — mirror of apps/api/internal/landing/types.go and
// apps/api/internal/booking/types.go. Kept hand-written (no codegen yet)
// to keep the dep surface small in Phase 1.

export type Locale = "th" | "en";

export type Branding = {
  logo_url?: string;
  primary_color?: string;
  accent_color?: string;
  font_family?: string;
};

export type SEO = {
  title?: string;
  description?: string;
  og_image_url?: string;
};

export type Tracking = {
  facebook_pixel_id?: string;
  google_analytics_id?: string;
  google_ads_conversion_id?: string;
  gtm_id?: string;
  line_tag_id?: string;
  tiktok_pixel_id?: string;
};

// Section content is opaque on the backend (validated only by `type`). The
// FE renders by `type`; per-type content shapes here are best-effort and
// every field is optional so partial content from the admin still renders.
export type SectionContentByType = {
  hero: {
    headline?: string;
    subheadline?: string;
    background_image_url?: string;
    cta_label?: string;
  };
  gallery: {
    images?: Array<{ url: string; alt?: string; caption?: string }>;
  };
  about: {
    title?: string;
    body?: string;
  };
  rooms: {
    title?: string;
    rooms?: Array<{
      id: string;
      name: string;
      description?: string;
      image_url?: string;
      max_occupancy?: number;
      base_rate?: string;
      base_currency?: string;
      amenities?: string[];
    }>;
  };
  amenities: {
    title?: string;
    items?: Array<{ icon?: string; label: string }>;
  };
  location: {
    title?: string;
    address?: string;
    latitude?: number;
    longitude?: number;
    directions?: string;
  };
  reviews: {
    title?: string;
    items?: Array<{
      author: string;
      rating: number;
      body: string;
      created_at?: string;
    }>;
  };
  faq: { items?: Array<{ q: string; a: string }> };
  policies: { body?: string };
  contact: { phone?: string; email?: string; line_id?: string };
};

export type SectionType = keyof SectionContentByType;

export type Section<T extends SectionType = SectionType> = {
  type: T;
  enabled: boolean;
  order: number;
  content: Partial<SectionContentByType[T]>;
};

export type PublicHotelContext = {
  name: string;
  slug: string;
  timezone: string;
  currency: string;
  promptpay_id?: string;
};

export type LandingPage = {
  id: string;
  hotel_id: string;
  locale: string;
  status: "draft" | "published";
  version: number;
  branding: Branding;
  sections: Section[];
  seo: SEO;
  tracking: Tracking;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
  hotel: PublicHotelContext;
};

// ----- pricing / quote -----

export type DailyRate = {
  date: string;
  amount: string;
  currency: string;
  applied_rules?: string[];
};

export type QuoteRequest = {
  room_type_id: string;
  check_in: string;
  check_out: string;
  rooms: number;
};

export type QuoteResponse = {
  hotel_id: string;
  room_type_id: string;
  rooms: number;
  nights: number;
  currency: string;
  per_night: DailyRate[];
  subtotal: string;
  total: string;
  adjustments?: string[];
};

// ----- booking -----

export type BookingStatus =
  | "pending_payment"
  | "confirmed"
  | "cancelled"
  | "expired"
  | "checked_in"
  | "checked_out"
  | "no_show"
  | "completed";

export type PaymentStatus =
  | "pending"
  | "paid"
  | "refunded"
  | "partial"
  | "failed";

export type Booking = {
  id: string;
  reference: string;
  hotel_id: string;
  room_type_id: string;
  room_count: number;
  guest_email: string;
  guest_phone?: string;
  guest_name: string;
  guest_country?: string;
  special_request?: string;
  check_in_date: string;
  check_out_date: string;
  nights: number;
  currency: string;
  room_subtotal_cents: number;
  taxes_cents: number;
  fees_cents: number;
  discounts_cents: number;
  total_cents: number;
  status: BookingStatus;
  payment_status: PaymentStatus;
  payment_method?: string;
  expires_at?: string | null;
  cancelled_at?: string | null;
  cancelled_by?: string;
  cancellation_reason?: string;
  source: "web" | "walk_in" | "phone" | "admin";
  created_at: string;
  updated_at: string;
  confirmed_at?: string | null;
  checked_in_at?: string | null;
  checked_out_at?: string | null;
};

// PublicBookingResponse flattens a Booking and adds the hotel context
// (timezone, currency, promptpay_id) the guest UI needs without a second
// round-trip. Returned by GET /v1/public/bookings/{reference} and the
// guest-side cancel endpoint. Backend uses Go struct embedding (allOf in
// the OpenAPI), so on the wire all Booking fields sit alongside `hotel`.
export type PublicBookingResponse = Booking & {
  hotel: PublicHotelContext;
};

export type BookingCreateRequest = {
  room_type_id: string;
  room_count: number;
  check_in_date: string;
  check_out_date: string;
  guest_email: string;
  guest_phone?: string;
  guest_name: string;
  guest_country?: string;
  special_request?: string;
  currency?: string;
  room_subtotal_cents?: number;
  taxes_cents?: number;
  fees_cents?: number;
  discounts_cents?: number;
  total_cents?: number;
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
  utm_term?: string;
  utm_content?: string;
  referrer?: string;
};

export type PublicCancelRequest = {
  email: string;
  reason?: string;
};
