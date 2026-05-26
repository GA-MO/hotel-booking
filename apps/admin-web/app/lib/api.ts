// API client: typed fetch wrapper around NEXT_PUBLIC_API_URL with bearer-auth
// and refresh-on-401. The refresh attempt is serialized — concurrent 401s
// share one in-flight refresh promise so we never spam the server.

import {
  clearAuth,
  getAccessToken,
  getRefreshToken,
  updateAccessToken,
  updateRefreshToken,
} from "./auth";
import type {
  ApiError,
  AuthResponse,
  AvailabilityResponse,
  Booking,
  BookingCreateRequest,
  BookingEvent,
  CreatePhotoRequest,
  CreatePricingRuleRequest,
  Hotel,
  HotelCreateRequest,
  HotelUpdateRequest,
  LandingPage,
  LandingUpdateRequest,
  Photo,
  PresignRequest,
  PresignResponse,
  PricingRule,
  RoomType,
  RoomTypeCreateRequest,
  RoomTypeUpdateRequest,
  Subscription,
  UpsertAvailabilityItem,
} from "./types";

const API_BASE =
  process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "") || "http://localhost:8080";

export class ApiClientError extends Error {
  code: string;
  status: number;
  constructor(status: number, code: string, message: string) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

let refreshInFlight: Promise<string | null> | null = null;

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight;
  const rt = getRefreshToken();
  if (!rt) return null;
  refreshInFlight = (async () => {
    try {
      const res = await fetch(`${API_BASE}/v1/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: rt }),
      });
      if (!res.ok) {
        clearAuth();
        return null;
      }
      const body = (await res.json()) as AuthResponse;
      updateAccessToken(body.access_token);
      updateRefreshToken(body.refresh_token);
      return body.access_token;
    } catch {
      clearAuth();
      return null;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  query?: Record<string, string | number | undefined | null>;
  // unauthenticated calls (signup/login/refresh)
  anonymous?: boolean;
  // expectNoContent: true → return null on 204
  expectNoContent?: boolean;
};

function buildURL(path: string, query?: RequestOptions["query"]): string {
  const url = new URL(`${API_BASE}${path}`);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null) continue;
      url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}

async function doFetch(path: string, opts: RequestOptions, retry = true): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Accept: "application/json",
  };
  if (!opts.anonymous) {
    const token = getAccessToken();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  const res = await fetch(buildURL(path, opts.query), {
    method: opts.method || "GET",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    credentials: "omit",
  });
  if (res.status === 401 && !opts.anonymous && retry) {
    const newToken = await refreshAccessToken();
    if (newToken) return doFetch(path, opts, false);
  }
  return res;
}

export async function apiRequest<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const res = await doFetch(path, opts);
  if (res.status === 204 || opts.expectNoContent) {
    return null as T;
  }
  const text = await res.text();
  let parsed: unknown = null;
  if (text.length > 0) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = null;
    }
  }
  if (!res.ok) {
    const err = parsed as ApiError | null;
    const code = err?.error?.code || `HTTP_${res.status}`;
    const message = err?.error?.message || res.statusText || "Request failed";
    throw new ApiClientError(res.status, code, message);
  }
  return parsed as T;
}

// ----- auth -----

export const Auth = {
  signup: (req: { email: string; password: string; name: string; locale?: string; country?: string }) =>
    apiRequest<AuthResponse>("/v1/auth/signup", { method: "POST", body: req, anonymous: true }),
  login: (req: { email: string; password: string }) =>
    apiRequest<AuthResponse>("/v1/auth/login", { method: "POST", body: req, anonymous: true }),
  logout: (refreshToken: string) =>
    apiRequest<null>("/v1/auth/logout", {
      method: "POST",
      body: { refresh_token: refreshToken },
      anonymous: true,
      expectNoContent: true,
    }),
  me: () => apiRequest<{ user: import("./types").User }>("/v1/auth/me"),
};

// ----- hotel -----

export const Hotels = {
  list: () => apiRequest<{ hotels: Hotel[] }>("/v1/hotels"),
  get: (id: string) => apiRequest<Hotel>(`/v1/hotels/${id}`),
  create: (req: HotelCreateRequest) =>
    apiRequest<Hotel>("/v1/hotels", { method: "POST", body: req }),
  update: (id: string, req: HotelUpdateRequest) =>
    apiRequest<Hotel>(`/v1/hotels/${id}`, { method: "PATCH", body: req }),
  slugAvailable: (slug: string) =>
    apiRequest<{ slug: string; available: boolean }>("/v1/hotels/slug-available", {
      query: { slug },
    }),
};

// ----- room types -----

export const RoomTypes = {
  list: (hotelID: string) =>
    apiRequest<{ room_types: RoomType[] }>(`/v1/hotels/${hotelID}/room-types`),
  get: (hotelID: string, id: string) =>
    apiRequest<RoomType>(`/v1/hotels/${hotelID}/room-types/${id}`),
  create: (hotelID: string, req: RoomTypeCreateRequest) =>
    apiRequest<RoomType>(`/v1/hotels/${hotelID}/room-types`, {
      method: "POST",
      body: req,
    }),
  update: (hotelID: string, id: string, req: RoomTypeUpdateRequest) =>
    apiRequest<RoomType>(`/v1/hotels/${hotelID}/room-types/${id}`, {
      method: "PATCH",
      body: req,
    }),
  remove: (hotelID: string, id: string) =>
    apiRequest<null>(`/v1/hotels/${hotelID}/room-types/${id}`, {
      method: "DELETE",
      expectNoContent: true,
    }),
};

// ----- landing -----

export const Landing = {
  list: (hotelID: string) =>
    apiRequest<{ landing_pages: LandingPage[] }>(`/v1/hotels/${hotelID}/landing`),
  get: (hotelID: string, locale: string) =>
    apiRequest<LandingPage>(`/v1/hotels/${hotelID}/landing/${locale}`),
  upsert: (hotelID: string, locale: string, req: LandingUpdateRequest) =>
    apiRequest<LandingPage>(`/v1/hotels/${hotelID}/landing/${locale}`, {
      method: "PUT",
      body: req,
    }),
  publish: (hotelID: string, locale: string) =>
    apiRequest<LandingPage>(`/v1/hotels/${hotelID}/landing/${locale}/publish`, {
      method: "POST",
    }),
  unpublish: (hotelID: string, locale: string) =>
    apiRequest<LandingPage>(`/v1/hotels/${hotelID}/landing/${locale}/unpublish`, {
      method: "POST",
    }),
};

// ----- bookings -----

export const Bookings = {
  list: (hotelID: string, query?: { status?: string; limit?: number }) =>
    apiRequest<{ bookings: Booking[]; total: number }>(
      `/v1/hotels/${hotelID}/bookings`,
      { query }
    ),
  get: (hotelID: string, id: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}`),
  create: (hotelID: string, req: BookingCreateRequest) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings`, { method: "POST", body: req }),
  confirm: (hotelID: string, id: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}/confirm`, { method: "POST" }),
  cancel: (hotelID: string, id: string, reason?: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}/cancel`, {
      method: "POST",
      body: { reason: reason || "" },
    }),
  checkIn: (hotelID: string, id: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}/check-in`, { method: "POST" }),
  checkOut: (hotelID: string, id: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}/check-out`, { method: "POST" }),
  noShow: (hotelID: string, id: string) =>
    apiRequest<Booking>(`/v1/hotels/${hotelID}/bookings/${id}/no-show`, { method: "POST" }),
  listEvents: (hotelID: string, id: string) =>
    apiRequest<{ events: BookingEvent[] }>(`/v1/hotels/${hotelID}/bookings/${id}/events`),
};

// ----- availability -----

export const Availability = {
  get: (hotelID: string, start: string, end: string) =>
    apiRequest<AvailabilityResponse>(`/v1/hotels/${hotelID}/availability`, {
      query: { start, end },
    }),
  upsert: (hotelID: string, items: UpsertAvailabilityItem[]) =>
    apiRequest<null>(`/v1/hotels/${hotelID}/availability`, {
      method: "PUT",
      body: { items },
      expectNoContent: true,
    }),
  remove: (hotelID: string, room_type_id: string, date: string) =>
    apiRequest<null>(`/v1/hotels/${hotelID}/availability`, {
      method: "DELETE",
      query: { room_type_id, date },
      expectNoContent: true,
    }),
};

// ----- pricing rules -----

export const Pricing = {
  list: (hotelID: string) =>
    apiRequest<{ rules: PricingRule[] }>(`/v1/hotels/${hotelID}/pricing-rules`),
  create: (hotelID: string, req: CreatePricingRuleRequest) =>
    apiRequest<PricingRule>(`/v1/hotels/${hotelID}/pricing-rules`, {
      method: "POST",
      body: req,
    }),
  update: (hotelID: string, id: string, req: Partial<CreatePricingRuleRequest>) =>
    apiRequest<PricingRule>(`/v1/hotels/${hotelID}/pricing-rules/${id}`, {
      method: "PATCH",
      body: req,
    }),
  remove: (hotelID: string, id: string) =>
    apiRequest<null>(`/v1/hotels/${hotelID}/pricing-rules/${id}`, {
      method: "DELETE",
      expectNoContent: true,
    }),
};

// ----- photos -----
//
// `storage_key` returned by Uploads.presign() flows into Photos.createRoomType /
// createHotel as-is. The backend stores it on the hotel_photos / room_type_photos
// row; imgproxy URL signing happens at render time via Uploads.imgproxyURL.

export const Photos = {
  listRoomType: (hotelID: string, roomTypeID: string) =>
    apiRequest<{ photos: Photo[] }>(
      `/v1/hotels/${hotelID}/room-types/${roomTypeID}/photos`,
    ),
  createRoomType: (hotelID: string, roomTypeID: string, req: CreatePhotoRequest) =>
    apiRequest<Photo>(`/v1/hotels/${hotelID}/room-types/${roomTypeID}/photos`, {
      method: "POST",
      body: req,
    }),
  removeRoomType: (hotelID: string, roomTypeID: string, photoID: string) =>
    apiRequest<null>(
      `/v1/hotels/${hotelID}/room-types/${roomTypeID}/photos/${photoID}`,
      { method: "DELETE", expectNoContent: true },
    ),
  listHotel: (hotelID: string) =>
    apiRequest<{ photos: Photo[] }>(`/v1/hotels/${hotelID}/photos`),
  createHotel: (hotelID: string, req: CreatePhotoRequest) =>
    apiRequest<Photo>(`/v1/hotels/${hotelID}/photos`, { method: "POST", body: req }),
  removeHotel: (hotelID: string, photoID: string) =>
    apiRequest<null>(`/v1/hotels/${hotelID}/photos/${photoID}`, {
      method: "DELETE",
      expectNoContent: true,
    }),
};

// ----- uploads -----
//
// Two-step browser flow:
//   1. POST /v1/uploads/presign -> { upload_url, object_key, public_url, headers }
//   2. PUT the file bytes to upload_url with the exact headers the response
//      lists; any deviation invalidates the SigV4 signature.
// Use Uploads.upload() to do both in one call.

export const Uploads = {
  presign: (req: PresignRequest) =>
    apiRequest<PresignResponse>("/v1/uploads/presign", { method: "POST", body: req }),

  async upload(file: File, kind: PresignRequest["kind"], hotelID?: string): Promise<PresignResponse> {
    const presigned = await Uploads.presign({
      kind,
      hotel_id: hotelID,
      content_type: file.type,
      size_bytes: file.size,
    });
    const putRes = await fetch(presigned.upload_url, {
      method: "PUT",
      body: file,
      headers: presigned.headers,
    });
    if (!putRes.ok) {
      throw new ApiClientError(putRes.status, "UPLOAD_FAILED", `storage rejected upload: ${putRes.status}`);
    }
    return presigned;
  },
};

// ----- subscription -----

export const Subs = {
  get: () => apiRequest<Subscription>("/v1/subscription"),
  cancel: (reason?: string) =>
    apiRequest<Subscription>("/v1/subscription/cancel", {
      method: "POST",
      body: { reason: reason || "" },
    }),
};
