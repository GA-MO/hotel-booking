"use client";

import { useEffect, useState } from "react";
import QRCode from "qrcode";

import type { Dict } from "../../../i18n";
import { ApiError, cancelBooking } from "../../../lib/api";
import { formatMoney, localeFor } from "../../../lib/money";
import { buildPromptPayPayload } from "../../../lib/promptpay";
import type { PublicBookingResponse } from "../../../lib/types";

type Props = {
  booking: PublicBookingResponse;
  locale: "th" | "en";
  dict: Dict;
  slug: string;
};

function canCancel(b: PublicBookingResponse) {
  return b.status === "pending_payment" || b.status === "confirmed";
}

// payloadFor returns a scannable QR string. When the hotel has a PromptPay ID
// configured we emit a real EMVCo PromptPay payload (mobile banking apps
// recognize it natively). Otherwise we fall back to a human-readable string
// so staff can still reconcile a payment by reference.
function payloadFor(b: PublicBookingResponse): string {
  if (b.hotel.promptpay_id) {
    const real = buildPromptPayPayload({
      promptpayID: b.hotel.promptpay_id,
      amountCents: b.total_cents,
    });
    if (real) return real;
  }
  return `PROMPTPAY|${b.reference}|${b.currency}|${b.total_cents}`;
}

export default function BookingActions({ booking, locale, dict, slug }: Props) {
  const [qrSrc, setQrSrc] = useState<string | null>(null);
  const [acknowledgedPaid, setAcknowledgedPaid] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [cancelledNow, setCancelledNow] = useState(false);

  useEffect(() => {
    let cancelled = false;
    QRCode.toDataURL(payloadFor(booking), { width: 240, margin: 1 })
      .then((src) => {
        if (!cancelled) setQrSrc(src);
      })
      .catch(() => {
        if (!cancelled) setQrSrc(null);
      });
    return () => {
      cancelled = true;
    };
  }, [booking]);

  const onCancel = async () => {
    if (!confirm(dict.confirmation.cancel_confirm)) return;
    const reason = prompt(dict.confirmation.cancel_reason_label) ?? undefined;
    setCancelling(true);
    setCancelError(null);
    try {
      await cancelBooking(booking.reference, {
        email: booking.guest_email,
        reason: reason || undefined,
      });
      setCancelledNow(true);
      // Force re-fetch so server-rendered status updates.
      window.location.reload();
    } catch (err) {
      if (err instanceof ApiError) {
        setCancelError(`${dict.common.error_generic} [${err.code}]`);
      } else {
        setCancelError(dict.common.error_generic);
      }
      setCancelling(false);
    }
  };

  const showPayment = booking.status === "pending_payment" && !cancelledNow;
  const cancelAllowed = canCancel(booking) && !cancelledNow;

  return (
    <div className="mt-8 space-y-6">
      {showPayment && (
        <section className="rounded-xl border border-neutral-200 bg-white p-6">
          <h2 className="mb-3 text-lg font-semibold">{dict.confirmation.payment_title}</h2>
          <p className="mb-4 text-sm text-neutral-600">{dict.confirmation.payment_hint}</p>

          <div className="flex flex-col items-center gap-4">
            {qrSrc ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={qrSrc} alt="" width={240} height={240} className="rounded-md" />
            ) : (
              <div className="h-[240px] w-[240px] animate-pulse rounded-md bg-neutral-100" />
            )}
            <p className="font-semibold">
              {formatMoney(booking.total_cents, booking.currency, localeFor(locale))}
            </p>
          </div>

          <div className="mt-6">
            {acknowledgedPaid ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                {dict.confirmation.payment_paid_thanks}
              </p>
            ) : (
              <button
                type="button"
                onClick={() => setAcknowledgedPaid(true)}
                className="w-full rounded-md bg-[var(--brand-primary,_#111)] px-4 py-3 text-sm font-medium text-white"
              >
                {dict.confirmation.payment_paid_button}
              </button>
            )}
          </div>
        </section>
      )}

      {cancelAllowed && (
        <section className="rounded-xl border border-neutral-200 bg-white p-6">
          <h2 className="mb-3 text-lg font-semibold">{dict.confirmation.cancel_title}</h2>
          {cancelError && (
            <p className="mb-3 rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-800">
              {cancelError}
            </p>
          )}
          <button
            type="button"
            onClick={onCancel}
            disabled={cancelling}
            className="rounded-md border border-red-300 px-4 py-2 text-sm font-medium text-red-700 hover:bg-red-50 disabled:opacity-50"
          >
            {dict.confirmation.cancel_button}
          </button>
        </section>
      )}

      <p className="text-center text-xs text-neutral-500">
        <a className="underline" href={`/${slug}?lang=${locale}`}>
          ← {slug}
        </a>
      </p>
    </div>
  );
}
