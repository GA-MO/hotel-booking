/** @type {import('next').NextConfig} */
const nextConfig = {
  // Standalone output produces a minimal Node server bundle in .next/standalone
  // required by the production Dockerfile (it copies that dir + .next/static + public).
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  images: {
    remotePatterns: [
      { protocol: "http",  hostname: "localhost" },
      { protocol: "https", hostname: "**.r2.dev" },
      { protocol: "https", hostname: "**.cloudflare.com" },
    ],
  },
  // typedRoutes intentionally disabled — booking-web routes everything through
  // the dynamic [slug] segment and the friction of casting/aliasing every
  // dynamic href outweighs the type-safety benefit at this stage.
};

export default nextConfig;
