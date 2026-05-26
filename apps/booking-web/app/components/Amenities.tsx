import type { Dict } from "../i18n";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"amenities">;
  dict: Dict;
};

export default function Amenities({ section, dict }: Props) {
  const items = section.content.items ?? [];
  const title = section.content.title ?? dict.landing.amenities_title;
  if (items.length === 0) return null;

  return (
    <section className="mx-auto max-w-5xl px-6 py-12">
      <h2 className="mb-6 text-2xl font-semibold">{title}</h2>
      <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
        {items.map((item, idx) => (
          <li
            key={`${item.label}-${idx}`}
            className="flex items-center gap-2 rounded-md border border-neutral-200 px-3 py-2 text-sm"
          >
            {item.icon && <span aria-hidden>{item.icon}</span>}
            <span>{item.label}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}
