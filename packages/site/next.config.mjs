import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { createMDX } from "fumadocs-mdx/next";

const withMDX = createMDX();
const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");

/** @type {import('next').NextConfig} */
const config = {
  // React Compiler (Next 16 stable): auto-memoization so the react-doctor
  // react-compiler-no-manual-memoization rule holds across web + ui + site.
  reactCompiler: true,
  reactStrictMode: true,
  trailingSlash: true,
  turbopack: {
    root: repoRoot,
  },
  images: {
    formats: ["image/avif", "image/webp"],
    qualities: [75, 90],
  },
};

export default withMDX(config);
