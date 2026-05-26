import type { Dict } from "../i18n";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"gallery">;
  dict: Dict;
};

export default function Gallery({ section, dict }: Props) {
  const images = section.content.images ?? [];
  if (images.length === 0) return null;

  return (
    <section className="mx-auto max-w-5xl px-6 py-12">
      <h2 className="mb-6 text-2xl font-semibold">{dict.landing.gallery_title}</h2>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
        {images.map((img, idx) => (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            key={`${img.url}-${idx}`}
            src={img.url}
            alt={img.alt ?? ""}
            className="aspect-[4/3] h-auto w-full rounded-lg object-cover"
            loading="lazy"
          />
        ))}
      </div>
    </section>
  );
}
