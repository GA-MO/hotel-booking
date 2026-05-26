import type { Dict } from "../i18n";
import { formatDecimal, localeFor } from "../lib/money";
import type { Section } from "../lib/types";

type Props = {
  section: Section<"rooms">;
  slug: string;
  dict: Dict;
  locale: "th" | "en";
};

export default function RoomList({ section, slug, dict, locale }: Props) {
  const rooms = section.content.rooms ?? [];
  const title = section.content.title ?? dict.landing.rooms_title;

  if (rooms.length === 0) return null;

  return (
    <section id="rooms" className="mx-auto max-w-5xl px-6 py-12">
      <h2 className="mb-6 text-2xl font-semibold">{title}</h2>
      <div className="grid gap-6 md:grid-cols-2">
        {rooms.map((room) => {
          const priceLabel =
            room.base_rate && room.base_currency
              ? formatDecimal(room.base_rate, room.base_currency, localeFor(locale))
              : null;
          return (
            <article
              key={room.id}
              className="overflow-hidden rounded-xl border border-neutral-200 bg-white"
            >
              {room.image_url ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  src={room.image_url}
                  alt={room.name}
                  className="aspect-[16/10] w-full object-cover"
                  loading="lazy"
                />
              ) : (
                <div className="aspect-[16/10] w-full bg-neutral-100" aria-hidden />
              )}
              <div className="p-5">
                <h3 className="text-lg font-semibold">{room.name}</h3>
                {room.description && (
                  <p className="mt-2 text-sm text-neutral-600">{room.description}</p>
                )}
                <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  {typeof room.max_occupancy === "number" && (
                    <div>
                      <dt className="text-neutral-500">{dict.landing.occupancy}</dt>
                      <dd className="font-medium">
                        {room.max_occupancy}{" "}
                        {room.max_occupancy === 1 ? dict.common.guest : dict.common.guests}
                      </dd>
                    </div>
                  )}
                  {priceLabel && (
                    <div>
                      <dt className="text-neutral-500">{dict.landing.from_price}</dt>
                      <dd className="font-medium">
                        {priceLabel} <span className="text-neutral-500">/ {dict.common.night}</span>
                      </dd>
                    </div>
                  )}
                </dl>
                <div className="mt-5">
                  <a
                    href={`/${slug}/book?lang=${locale}&room_type_id=${encodeURIComponent(room.id)}`}
                    className="inline-flex items-center justify-center rounded-md bg-[var(--brand-primary,_#111)] px-4 py-2 text-sm font-medium text-white"
                  >
                    {dict.landing.select_this_room}
                  </a>
                </div>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
