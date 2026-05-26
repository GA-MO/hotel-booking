import type { Dict } from "../i18n";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"reviews">;
  dict: Dict;
};

function Stars({ rating }: { rating: number }) {
  const clamped = Math.max(0, Math.min(5, Math.round(rating)));
  return (
    <span aria-label={`${rating} / 5`} className="text-amber-500">
      {"★".repeat(clamped)}
      <span className="text-neutral-300">{"★".repeat(5 - clamped)}</span>
    </span>
  );
}

export default function Reviews({ section, dict }: Props) {
  const items = section.content.items ?? [];
  const title = section.content.title ?? dict.landing.reviews_title;
  if (items.length === 0) return null;

  return (
    <section className="mx-auto max-w-5xl px-6 py-12">
      <h2 className="mb-6 text-2xl font-semibold">{title}</h2>
      <ul className="grid gap-4 md:grid-cols-2">
        {items.map((r, idx) => (
          <li
            key={`${r.author}-${idx}`}
            className="rounded-lg border border-neutral-200 bg-white p-5"
          >
            <div className="flex items-center justify-between">
              <p className="font-medium">{r.author}</p>
              <Stars rating={r.rating} />
            </div>
            <p className="mt-3 text-sm text-neutral-700">{r.body}</p>
          </li>
        ))}
      </ul>
    </section>
  );
}
