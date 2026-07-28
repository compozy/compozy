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
};

export default withMDX(config);
