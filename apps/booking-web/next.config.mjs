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
  experimental: {
    typedRoutes: true,
  },
};

export default nextConfig;
