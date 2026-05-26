import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Hotel Booking",
  description: "Direct booking for small hotels",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  // The `lang` attribute is set to the platform default; per-page locale is
  // already applied through Intl formatters and dictionaries, so the static
  // root attribute only matters for screen readers on the root error pages.
  return (
    <html lang="en">
      <body className="min-h-screen bg-white font-sans text-neutral-900 antialiased">
        {children}
      </body>
    </html>
  );
}
