"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import type { Dict } from "../../i18n";
import { ApiError, createBooking, postQuote } from "../../lib/api";
import { addDays, isYMD, nightsBetween, todayYMD } from "../../lib/dates";
import { formatDecimal, localeFor } from "../../lib/money";
import type { QuoteResponse } from "../../lib/types";

type Room = {
  id: string;
  name: string;
  base_rate?: string;
  base_currency?: string;
  max_occupancy?: number;
};

type Props = {
  slug: string;
  locale: "th" | "en";
  dict: Dict;
  rooms: Room[];
  initialRoomId?: string;
};

export default function BookForm({ slug, locale, dict, rooms, initialRoomId }: Props) {
  const today = todayYMD();
  const tomorrow = addDays(today, 1);

  const initialRoom =
    rooms.find((r) => r.id === initialRoomId)?.id ?? rooms[0]?.id ?? "";

  const [roomTypeId, setRoomTypeId] = useState(initialRoom);
  const [checkIn, setCheckIn] = useState(today);
  const [checkOut, setCheckOut] = useState(tomorrow);
  const [roomCount, setRoomCount] = useState(1);
  const [guestName, setGuestName] = useState("");
  const [guestEmail, setGuestEmail] = useState("");
  const [guestPhone, setGuestPhone] = useState("");
  const [specialRequest, setSpecialRequest] = useState("");

  const [quote, setQuote] = useState<QuoteResponse | null>(null);
  const [quoteLoading, setQuoteLoading] = useState(false);
  const [quoteError, setQuoteError] = useState<string | null>(null);

  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const nights = nightsBetween(checkIn, checkOut);
  const datesValid = isYMD(checkIn) && isYMD(checkOut) && nights >= 1;

  // Debounced quote fetching so each keystroke on dates/rooms doesn't fire
  // a request. `quoteSeq` invalidates in-flight responses if newer inputs
  // start a fresh quote — last-writer-wins.
  const quoteSeq = useRef(0);
  const fetchQuote = useCallback(async () => {
    if (!roomTypeId || !datesValid) {
      setQuote(null);
      return;
    }
    const seq = ++quoteSeq.current;
    setQuoteLoading(true);
    setQuoteError(null);
    try {
      const res = await postQuote(slug, {
        room_type_id: roomTypeId,
        check_in: checkIn,
        check_out: checkOut,
        rooms: roomCount,
      });
      if (seq === quoteSeq.current) {
        setQuote(res);
      }
    } catch (err) {
      if (seq !== quoteSeq.current) return;
      if (err instanceof ApiError && (err.status === 400 || err.status === 404 || err.status === 409)) {
        setQuoteError(dict.book.quote_unavailable);
      } else {
        setQuoteError(dict.common.error_generic);
      }
      setQuote(null);
    } finally {
      if (seq === quoteSeq.current) setQuoteLoading(false);
    }
  }, [slug, roomTypeId, checkIn, checkOut, roomCount, datesValid, dict]);

  useEffect(() => {
    const t = setTimeout(fetchQuote, 250);
    return () => clearTimeout(t);
  }, [fetchQuote]);

  const minCheckOut = useMemo(() => addDays(isYMD(checkIn) ? checkIn : today, 1), [checkIn, today]);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!roomTypeId) {
      setSubmitError(dict.book.pick_room_first);
      return;
    }
    if (!datesValid) {
      setSubmitError(dict.book.invalid_dates);
      return;
    }
    if (!guestName.trim() || !guestEmail.trim()) {
      setSubmitError(dict.common.error_generic);
      return;
    }

    setSubmitting(true);
    setSubmitError(null);
    try {
      const booking = await createBooking(slug, {
        room_type_id: roomTypeId,
        room_count: roomCount,
        check_in_date: checkIn,
        check_out_date: checkOut,
        guest_email: guestEmail.trim(),
        guest_name: guestName.trim(),
        guest_phone: guestPhone.trim() || undefined,
        special_request: specialRequest.trim() || undefined,
      });
      // Preserve email in query so the confirmation page can fetch the
      // booking without re-prompting. The link is the only place we keep it.
      const q = new URLSearchParams({ lang: locale, email: booking.guest_email });
      window.location.href = `/${slug}/booking/${encodeURIComponent(booking.reference)}?${q.toString()}`;
    } catch (err) {
      if (err instanceof ApiError) {
        setSubmitError(`${dict.common.error_generic} [${err.code}]`);
      } else {
        setSubmitError(dict.common.error_generic);
      }
      setSubmitting(false);
    }
  };

  const formatPrice = (amount: string, currency: string) =>
    formatDecimal(amount, currency, localeFor(locale));

  return (
    <form onSubmit={onSubmit} className="space-y-8">
      <fieldset className="space-y-4 rounded-lg border border-neutral-200 p-5">
        <div className="grid gap-4 sm:grid-cols-2">
          <label className="flex flex-col text-sm">
            <span className="mb-1 font-medium">{dict.book.check_in}</span>
            <input
              type="date"
              required
              value={checkIn}
              min={today}
              onChange={(e) => {
                setCheckIn(e.target.value);
                if (e.target.value >= checkOut) {
                  setCheckOut(addDays(e.target.value, 1));
                }
              }}
              className="rounded-md border border-neutral-300 px-3 py-2"
            />
          </label>

          <label className="flex flex-col text-sm">
            <span className="mb-1 font-medium">{dict.book.check_out}</span>
            <input
              type="date"
              required
              value={checkOut}
              min={minCheckOut}
              onChange={(e) => setCheckOut(e.target.value)}
              className="rounded-md border border-neutral-300 px-3 py-2"
            />
          </label>

          <label className="flex flex-col text-sm">
            <span className="mb-1 font-medium">{dict.book.room_type}</span>
            <select
              required
              value={roomTypeId}
              onChange={(e) => setRoomTypeId(e.target.value)}
              className="rounded-md border border-neutral-300 px-3 py-2"
            >
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </select>
          </label>

          <label className="flex flex-col text-sm">
            <span className="mb-1 font-medium">{dict.book.rooms}</span>
            <input
              type="number"
              min={1}
              max={10}
              value={roomCount}
              onChange={(e) => setRoomCount(Math.max(1, Number(e.target.value) || 1))}
              className="rounded-md border border-neutral-300 px-3 py-2"
            />
          </label>
        </div>

        {!datesValid && (
          <p className="text-sm text-amber-700">{dict.book.invalid_dates}</p>
        )}
      </fieldset>

      <fieldset className="space-y-4 rounded-lg border border-neutral-200 p-5">
        <legend className="px-2 text-sm font-semibold">{dict.book.guest_details}</legend>

        <label className="flex flex-col text-sm">
          <span className="mb-1 font-medium">{dict.book.name}</span>
          <input
            type="text"
            required
            value={guestName}
            onChange={(e) => setGuestName(e.target.value)}
            className="rounded-md border border-neutral-300 px-3 py-2"
            autoComplete="name"
          />
        </label>

        <label className="flex flex-col text-sm">
          <span className="mb-1 font-medium">{dict.book.email}</span>
          <input
            type="email"
            required
            value={guestEmail}
            onChange={(e) => setGuestEmail(e.target.value)}
            className="rounded-md border border-neutral-300 px-3 py-2"
            autoComplete="email"
          />
        </label>

        <label className="flex flex-col text-sm">
          <span className="mb-1 font-medium">{dict.book.phone}</span>
          <input
            type="tel"
            value={guestPhone}
            onChange={(e) => setGuestPhone(e.target.value)}
            className="rounded-md border border-neutral-300 px-3 py-2"
            autoComplete="tel"
          />
        </label>

        <label className="flex flex-col text-sm">
          <span className="mb-1 font-medium">{dict.book.special_requests}</span>
          <textarea
            value={specialRequest}
            onChange={(e) => setSpecialRequest(e.target.value)}
            rows={3}
            className="rounded-md border border-neutral-300 px-3 py-2"
          />
        </label>
      </fieldset>

      <section
        aria-live="polite"
        className="rounded-lg border border-neutral-200 bg-neutral-50 p-5"
      >
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-wide text-neutral-500">
          {dict.book.summary_title}
        </h2>

        {!datesValid ? (
          <p className="text-sm text-neutral-600">{dict.book.quote_select_dates}</p>
        ) : quoteLoading ? (
          <p className="text-sm text-neutral-600">{dict.book.quote_loading}</p>
        ) : quoteError ? (
          <p className="text-sm text-red-700">{quoteError}</p>
        ) : quote ? (
          <dl className="space-y-2 text-sm">
            <div className="flex justify-between">
              <dt className="text-neutral-600">
                {nights} {nights === 1 ? dict.common.night : dict.common.nights}
                {" · "}
                {roomCount} {roomCount === 1 ? dict.common.room : dict.common.rooms}
              </dt>
              <dd>{formatPrice(quote.subtotal, quote.currency)}</dd>
            </div>
            <div className="flex justify-between border-t border-neutral-200 pt-2 text-base font-semibold">
              <dt>{dict.book.quote_total}</dt>
              <dd>{formatPrice(quote.total, quote.currency)}</dd>
            </div>
          </dl>
        ) : null}
      </section>

      {submitError && (
        <p className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          {submitError}
        </p>
      )}

      <button
        type="submit"
        disabled={submitting || !quote || !datesValid}
        className="w-full rounded-md bg-[var(--brand-primary,_#111)] px-4 py-3 text-base font-medium text-white shadow disabled:cursor-not-allowed disabled:opacity-50"
      >
        {submitting ? dict.book.submitting : dict.book.submit}
      </button>
    </form>
  );
}
