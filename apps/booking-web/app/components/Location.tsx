import type { Dict } from "../i18n";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"location">;
  dict: Dict;
};

export default function Location({ section, dict }: Props) {
  const c = section.content;
  const title = c.title ?? dict.landing.location_title;
  const hasMap = typeof c.latitude === "number" && typeof c.longitude === "number";

  if (!c.address && !hasMap && !c.directions) return null;

  return (
    <section className="mx-auto max-w-5xl px-6 py-12">
      <h2 className="mb-6 text-2xl font-semibold">{title}</h2>
      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-3 text-neutral-700">
          {c.address && <p className="whitespace-pre-line">{c.address}</p>}
          {c.directions && (
            <p className="whitespace-pre-line text-sm text-neutral-600">{c.directions}</p>
          )}
        </div>
        {hasMap && (
          // OpenStreetMap embed avoids requiring a Google Maps API key in
          // Phase 1; admin still customizes lat/lng on the landing page.
          <iframe
            title={title}
            className="aspect-video w-full rounded-lg border border-neutral-200"
            src={`https://www.openstreetmap.org/export/embed.html?bbox=${
              (c.longitude as number) - 0.005
            }%2C${(c.latitude as number) - 0.003}%2C${
              (c.longitude as number) + 0.005
            }%2C${(c.latitude as number) + 0.003}&layer=mapnik&marker=${c.latitude}%2C${c.longitude}`}
            loading="lazy"
          />
        )}
      </div>
    </section>
  );
}
