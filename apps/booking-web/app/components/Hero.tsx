import type { Dict } from "../i18n";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"hero">;
  slug: string;
  dict: Dict;
  locale: "th" | "en";
};

export default function Hero({ section, slug, dict, locale }: Props) {
  const c = section.content;
  const headline = c.headline ?? "";
  const sub = c.subheadline ?? "";
  const bg = c.background_image_url;
  const cta = c.cta_label ?? dict.common.book_now;

  return (
    <section className="relative isolate overflow-hidden">
      {bg ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={bg}
          alt=""
          className="absolute inset-0 -z-10 h-full w-full object-cover"
          loading="eager"
        />
      ) : (
        <div className="absolute inset-0 -z-10 bg-neutral-200" aria-hidden />
      )}
      <div className="absolute inset-0 -z-10 bg-black/40" aria-hidden />
      <div className="mx-auto flex min-h-[60vh] max-w-5xl flex-col justify-end px-6 py-16 text-white">
        {headline && (
          <h1 className="text-4xl font-semibold tracking-tight md:text-6xl">{headline}</h1>
        )}
        {sub && <p className="mt-4 max-w-2xl text-lg text-white/90">{sub}</p>}
        <div className="mt-8">
          <a
            href={`/${slug}/book?lang=${locale}`}
            className="inline-flex items-center justify-center rounded-full bg-[var(--brand-primary,_#111)] px-6 py-3 text-base font-medium text-white shadow transition hover:opacity-90"
          >
            {cta}
          </a>
        </div>
      </div>
    </section>
  );
}
