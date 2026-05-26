import { notFound } from "next/navigation";

type Params = { slug: string };

// Phase 1 will:
//   - fetch hotel + landing_page config from API by slug
//   - render Hero, Gallery, Rooms, Amenities, Location, Reviews sections
//   - use ISR via `export const revalidate = ...`
//   - fire tracking pixels per landing.tracking config
//
// Phase 0 stub: prove the [slug] route resolves and the data fetch boundary exists.

async function loadHotel(_slug: string): Promise<{ name: string } | null> {
  // TODO: replace with API call e.g. fetch(`${API_URL}/v1/landing/${slug}`, { next: { revalidate: 60 } })
  return null;
}

export const revalidate = 60;

export default async function HotelLandingPage({
  params,
}: {
  params: Promise<Params>;
}) {
  const { slug } = await params;
  const hotel = await loadHotel(slug);
  if (!hotel) notFound();

  return (
    <main className="mx-auto max-w-5xl px-6 py-16">
      <h1 className="text-4xl font-semibold tracking-tight">{hotel.name}</h1>
      <p className="mt-4 text-neutral-600">Slug: {slug}</p>
    </main>
  );
}
