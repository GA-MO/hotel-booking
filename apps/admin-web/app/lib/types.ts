// Mirrors apps/api/internal/*/types.go DTOs. Hand-traced from docs/api/openapi.yaml.

export type ApiError = {
  error: { code: string; message: string };
};

export type Role = "owner" | "manager" | "front_desk" | "read_only";

export type User = {
  id: string;
  account_id: string;
  email: string;
  name: string;
  role: Role;
  locale: string;
  email_verified_at?: string | null;
  created_at: string;
};

export type AuthResponse = {
  user: User;
  access_token: string;
  refresh_token: string;
  access_token_expires_in: number;
};

export type SignupRequest = {
  email: string;
  password: string;
  name: string;
  locale?: string;
  country?: string;
};

export type LoginRequest = { email: string; password: string };

export type HotelKYCStatus = "pending" | "submitted" | "approved" | "rejected";
export type HotelStatus = "test" | "live" | "suspended" | "archived";

export type Hotel = {
  id: string;
  account_id: string;
  slug: string;
  name: string;
  hotel_type?: string;
  description?: string;
  address_line?: string;
  city?: string;
  country?: string;
  postal_code?: string;
  latitude?: number | null;
  longitude?: number | null;
  phone?: string;
  email?: string;
  line_id?: string;
  timezone: string;
  base_currency: string;
  check_in_time: string;
  check_out_time: string;
  promptpay_id?: string | null;
  kyc_status: HotelKYCStatus;
  status: HotelStatus;
  created_at: string;
  updated_at: string;
};

export type HotelCreateRequest = {
  slug: string;
  name: string;
  hotel_type?: string;
  country?: string;
  timezone?: string;
  base_currency?: string;
};

export type HotelUpdateRequest = Partial<{
  name: string;
  hotel_type: string;
  description: string;
  address_line: string;
  city: string;
  country: string;
  postal_code: string;
  latitude: number | null;
  longitude: number | null;
  phone: string;
  email: string;
  line_id: string;
  timezone: string;
  base_currency: string;
  check_in_time: string;
  check_out_time: string;
  promptpay_id: string | null;
}>;

export type RoomType = {
  id: string;
  hotel_id: string;
  name: string;
  description?: string;
  total_inventory: number;
  max_occupancy: number;
  size_sqm?: number | null;
  bed_config: unknown;
  amenities: unknown;
  base_rate: number;
  base_currency: string;
  display_order: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type RoomTypeCreateRequest = {
  name: string;
  description?: string;
  total_inventory: number;
  max_occupancy: number;
  size_sqm?: number | null;
  bed_config?: unknown;
  amenities?: unknown;
  base_rate: number;
  base_currency?: string;
  display_order?: number;
};

export type RoomTypeUpdateRequest = Partial<RoomTypeCreateRequest & { enabled: boolean }>;

// ----- photos + uploads -----

export type Photo = {
  id: string;
  hotel_id?: string | null;
  room_type_id?: string | null;
  storage_key: string;
  caption?: string;
  alt_text?: string;
  width?: number | null;
  height?: number | null;
  display_order: number;
  is_cover: boolean;
  created_at: string;
};

export type CreatePhotoRequest = {
  storage_key: string;
  caption?: string;
  alt_text?: string;
  width?: number | null;
  height?: number | null;
  display_order?: number;
  is_cover?: boolean;
};

export type UploadKind = "hotel_photo" | "room_type_photo";

export type PresignRequest = {
  kind: UploadKind;
  hotel_id?: string;
  content_type: string;
  size_bytes: number;
};

export type PresignResponse = {
  upload_url: string;
  object_key: string;
  public_url: string;
  headers: Record<string, string>;
  expires_at: string;
};

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
export type LandingSection = {
  type: string;
  enabled: boolean;
  order: number;
  content: Record<string, unknown>;
};
export type LandingPage = {
  id: string;
  hotel_id: string;
  locale: string;
  status: "draft" | "published";
  version: number;
  branding: Branding;
  sections: LandingSection[];
  seo: SEO;
  tracking: Tracking;
  published_at?: string | null;
  created_at: string;
  updated_at: string;
};
export type LandingUpdateRequest = {
  branding: Branding;
  sections: LandingSection[];
  seo: SEO;
  tracking: Tracking;
};

export type BookingStatus =
  | "pending_payment"
  | "confirmed"
  | "cancelled"
  | "expired"
  | "checked_in"
  | "checked_out"
  | "no_show"
  | "completed";

export type BookingPaymentStatus =
  | "pending"
  | "paid"
  | "refunded"
  | "partial"
  | "failed";

export type BookingSource = "web" | "walk_in" | "phone" | "admin";

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
  payment_status: BookingPaymentStatus;
  payment_method?: string;
  expires_at?: string | null;
  cancelled_at?: string | null;
  cancelled_by?: string;
  cancellation_reason?: string;
  source: BookingSource;
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
  utm_term?: string;
  utm_content?: string;
  referrer?: string;
  created_at: string;
  updated_at: string;
  confirmed_at?: string | null;
  checked_in_at?: string | null;
  checked_out_at?: string | null;
};

// BookingEvent is one row from the append-only audit log. Powers the
// admin booking-detail timeline. `payload` is opaque JSON per event type.
export type BookingEvent = {
  id: string;
  booking_id: string;
  event_type: string;
  actor_type: "system" | "hotel_staff" | "guest";
  actor_id?: string | null;
  payload: Record<string, unknown>;
  created_at: string;
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
};

export type AvailabilityDay = {
  date: string;
  room_type_id: string;
  total_inventory: number;
  available: number;
  closed: boolean;
  rate: string;
  currency: string;
  min_nights?: number | null;
  max_nights?: number | null;
  closed_to_arrival: boolean;
  closed_to_departure: boolean;
};

export type AvailabilityResponse = {
  hotel_id: string;
  start: string;
  end: string;
  days: AvailabilityDay[];
};

export type UpsertAvailabilityItem = {
  room_type_id: string;
  date: string;
  inventory_change?: number;
  closed?: boolean;
  rate_override?: string | null;
  rate_currency?: string | null;
  min_nights?: number | null;
  max_nights?: number | null;
  closed_to_arrival?: boolean;
  closed_to_departure?: boolean;
};

export type PricingRuleType =
  | "season"
  | "day_of_week"
  | "length_of_stay"
  | "advance_purchase";

export type PricingModifierType = "percentage" | "fixed_amount" | "set_value";

export type PricingRule = {
  id: string;
  hotel_id: string;
  room_type_id?: string | null;
  name: string;
  rule_type: PricingRuleType;
  start_date?: string | null;
  end_date?: string | null;
  days_of_week?: number[];
  modifier_type: PricingModifierType;
  modifier_value: string;
  min_nights?: number | null;
  max_nights?: number | null;
  min_days_ahead?: number | null;
  max_days_ahead?: number | null;
  priority: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type CreatePricingRuleRequest = {
  room_type_id?: string | null;
  name: string;
  rule_type: PricingRuleType;
  start_date?: string | null;
  end_date?: string | null;
  days_of_week?: number[];
  modifier_type: PricingModifierType;
  modifier_value: string;
  min_nights?: number | null;
  max_nights?: number | null;
  min_days_ahead?: number | null;
  max_days_ahead?: number | null;
  priority?: number | null;
  enabled?: boolean | null;
};

export type SubscriptionStatus =
  | "pending_kyc"
  | "trialing"
  | "trial_ending"
  | "trial_lapsed"
  | "active"
  | "past_due"
  | "suspended"
  | "cancelled"
  | "terminated";

export type Subscription = {
  id: string;
  account_id: string;
  status: SubscriptionStatus;
  plan_code?: string;
  billing_cycle?: string;
  trial_started_at?: string | null;
  trial_ends_at?: string | null;
  current_period_start?: string | null;
  current_period_end?: string | null;
  room_count_snapshot?: number | null;
  unit_price_cents?: number | null;
  currency?: string;
  payment_provider?: string;
  payment_method_id?: string;
  payment_method_last4?: string;
  payment_method_brand?: string;
  cancelled_at?: string | null;
  cancellation_reason?: string;
  suspended_at?: string | null;
  created_at: string;
  updated_at: string;
};
