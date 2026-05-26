import type {
  Booking,
  BookingCreateRequest,
  LandingPage,
  PublicCancelRequest,
  QuoteRequest,
  QuoteResponse,
} from "./types";

// API error envelope shape from apps/api/internal/platform/respond.
// `code` is a stable machine code (NOT_FOUND, BAD_REQUEST, ...); `message`
// is human-readable but not user-safe — never surface it raw to guests.
export type ApiErrorBody = {
  error: { code: string; message: string };
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly body: ApiErrorBody | null;

  constructor(status: number, code: string, message: string, body: ApiErrorBody | null) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.body = body;
  }

  isNotFound(): boolean {
    return this.status === 404;
  }
}

function apiBase(): string {
  // Server-side: prefer NEXT_PUBLIC_API_URL (same env var used FE-side in Next).
  // We standardize on the single var to keep parity between SSR and CSR fetches.
  const url = process.env.NEXT_PUBLIC_API_URL;
  if (!url) {
    throw new Error("NEXT_PUBLIC_API_URL is not configured");
  }
  return url.replace(/\/+$/, "");
}

type FetchOpts = {
  // Next.js fetch options for caching control.
  revalidate?: number | false;
  cache?: RequestCache;
  signal?: AbortSignal;
};

async function request<T>(
  path: string,
  init: RequestInit,
  opts: FetchOpts = {},
): Promise<T> {
  const url = `${apiBase()}${path}`;
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  headers.set("Accept", "application/json");

  const res = await fetch(url, {
    ...init,
    headers,
    signal: opts.signal,
    cache: opts.cache,
    next:
      opts.revalidate === undefined
        ? undefined
        : { revalidate: opts.revalidate === false ? 0 : opts.revalidate },
  });

  if (!res.ok) {
    let body: ApiErrorBody | null = null;
    try {
      body = (await res.json()) as ApiErrorBody;
    } catch {
      body = null;
    }
    const code = body?.error?.code ?? "UNKNOWN";
    const message = body?.error?.message ?? res.statusText ?? "request failed";
    throw new ApiError(res.status, code, message, body);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// ----- public endpoints used by booking-web -----

export function getLanding(slug: string, locale: string): Promise<LandingPage> {
  // ISR-friendly: re-validate hourly. Pages also set `revalidate=60` at the
  // route level so this is a backstop.
  return request<LandingPage>(
    `/v1/public/landing/${encodeURIComponent(slug)}/${encodeURIComponent(locale)}`,
    { method: "GET" },
    { revalidate: 3600 },
  );
}

export function postQuote(slug: string, body: QuoteRequest): Promise<QuoteResponse> {
  return request<QuoteResponse>(
    `/v1/public/quote/${encodeURIComponent(slug)}`,
    { method: "POST", body: JSON.stringify(body) },
    { cache: "no-store" },
  );
}

export function createBooking(
  slug: string,
  body: BookingCreateRequest,
): Promise<Booking> {
  return request<Booking>(
    `/v1/public/hotels/${encodeURIComponent(slug)}/bookings`,
    { method: "POST", body: JSON.stringify(body) },
    { cache: "no-store" },
  );
}

export function getBooking(reference: string, email: string): Promise<Booking> {
  const q = new URLSearchParams({ email });
  return request<Booking>(
    `/v1/public/bookings/${encodeURIComponent(reference)}?${q.toString()}`,
    { method: "GET" },
    { cache: "no-store" },
  );
}

export function cancelBooking(
  reference: string,
  body: PublicCancelRequest,
): Promise<Booking> {
  return request<Booking>(
    `/v1/public/bookings/${encodeURIComponent(reference)}/cancel`,
    { method: "POST", body: JSON.stringify(body) },
    { cache: "no-store" },
  );
}
