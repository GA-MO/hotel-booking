import type { Dict } from "../i18n";
import type { Section, SectionType } from "../lib/types";

import Amenities from "./Amenities";
import Gallery from "./Gallery";
import Hero from "./Hero";
import Location from "./Location";
import Reviews from "./Reviews";
import RoomList from "./RoomList";

type Props = {
  sections: Section[];
  slug: string;
  dict: Dict;
  locale: "th" | "en";
};

// SUPPORTED is the FE-side whitelist of section types we know how to render.
// Other types in the API catalog (about, faq, policies, contact) are read by
// the backend but intentionally skipped here — adding them is additive.
const SUPPORTED: ReadonlySet<SectionType> = new Set<SectionType>([
  "hero",
  "gallery",
  "rooms",
  "amenities",
  "location",
  "reviews",
]);

export default function SectionRenderer({ sections, slug, dict, locale }: Props) {
  const ordered = [...sections]
    .filter((s) => s.enabled && SUPPORTED.has(s.type as SectionType))
    .sort((a, b) => a.order - b.order);

  return (
    <>
      {ordered.map((s, idx) => {
        const key = `${s.type}-${idx}`;
        switch (s.type) {
          case "hero":
            return (
              <Hero
                key={key}
                section={s as Section<"hero">}
                slug={slug}
                dict={dict}
                locale={locale}
              />
            );
          case "gallery":
            return <Gallery key={key} section={s as Section<"gallery">} dict={dict} />;
          case "rooms":
            return (
              <RoomList
                key={key}
                section={s as Section<"rooms">}
                slug={slug}
                dict={dict}
                locale={locale}
              />
            );
          case "amenities":
            return <Amenities key={key} section={s as Section<"amenities">} dict={dict} />;
          case "location":
            return <Location key={key} section={s as Section<"location">} dict={dict} />;
          case "reviews":
            return <Reviews key={key} section={s as Section<"reviews">} dict={dict} />;
          default:
            return null;
        }
      })}
    </>
  );
}
