import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  reactStrictMode: true,
  async redirects() {
    return [
      {
        source: "/docs",
        destination: "/docs/core",
        permanent: true,
      },
      { source: "/code", destination: "https://code.compozy.com", permanent: true },
      { source: "/code/:path*", destination: "https://code.compozy.com/:path*", permanent: true },
    ];
  },
  async rewrites() {
    return [
      {
        source: "/docs/:path*.mdx",
        destination: "/llms.mdx/:path*",
      },
    ];
  },
  images: {
    remotePatterns: [
      {
        protocol: "https",
        hostname: "ik.imagekit.io",
        pathname: "/**",
      },
    ],
  },
  serverExternalPackages: ["ts-morph", "typescript", "oxc-transform", "twoslash", "shiki"],
};

export default withMDX(config);
