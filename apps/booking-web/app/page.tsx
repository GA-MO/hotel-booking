// Root path is intentionally minimal — this platform is direct-booking only.
// Guests arrive on /[slug] directly from ads / social. No discovery / search here.
export default function HomePage() {
  return (
    <main className="mx-auto flex min-h-screen max-w-2xl flex-col items-center justify-center px-6 text-center">
      <h1 className="text-3xl font-semibold tracking-tight">Hotel Booking</h1>
      <p className="mt-3 text-neutral-600">
        This is a direct booking platform. Visit your hotel&apos;s unique link to book.
      </p>
    </main>
  );
}
