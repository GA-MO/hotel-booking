/** @type {import('next').NextConfig} */
const nextConfig = {
  // Standalone output produces a minimal Node server bundle in .next/standalone
  // required by the production Dockerfile (it copies that dir + .next/static + public).
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  typedRoutes: true,
};

export default nextConfig;
