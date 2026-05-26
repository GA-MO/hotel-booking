"use client";

import { useState } from "react";

import type { Dict } from "../../../i18n";

type Props = {
  reference: string;
  initialEmail: string;
  dict: Dict;
  locale: "th" | "en";
  slug: string;
};

// LookupForm is the fallback shown when the visitor lands on a booking
// confirmation URL without an `?email=` query param. Submitting redirects
// to the same page with the email attached, which lets the server-side
// fetch resolve the booking.
export default function LookupForm({ reference, initialEmail, dict, locale, slug }: Props) {
  const [email, setEmail] = useState(initialEmail);

  const onSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = new URLSearchParams({ lang: locale, email: email.trim() });
    window.location.href = `/${slug}/booking/${encodeURIComponent(reference)}?${q.toString()}`;
  };

  return (
    <form onSubmit={onSubmit} className="space-y-4">
      <label className="flex flex-col text-sm">
        <span className="mb-1 font-medium">{dict.book.email}</span>
        <input
          type="email"
          required
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="rounded-md border border-neutral-300 px-3 py-2"
          autoComplete="email"
        />
      </label>
      <button
        type="submit"
        className="rounded-md bg-[var(--brand-primary,_#111)] px-4 py-2 text-sm font-medium text-white"
      >
        {dict.confirmation.look_up_button}
      </button>
    </form>
  );
}
