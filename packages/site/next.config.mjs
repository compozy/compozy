import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();

/** @type {import('next').NextConfig} */
const config = {
  // React Compiler (Next 16 stable): auto-memoization so the react-doctor
  // react-compiler-no-manual-memoization rule holds across web + ui + site.
  reactCompiler: true,
  reactStrictMode: true,
  trailingSlash: true,
  images: {
    formats: ["image/avif", "image/webp"],
    qualities: [75, 90],
  },
  async redirects() {
    return [
      {
        source: "/blog/introducing-agh-the-first-agent-network-protocol",
        destination: "/blog/introducing-compozy-the-first-agent-network-protocol",
        permanent: true,
      },
    ];
  },
};

export default withMDX(config);
