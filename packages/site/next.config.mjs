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
  // D7: the /runtime + /protocol trees moved under /docs (site IA spec §5.3). These 301s are the
  // only sanctioned bridge — scheduled delete one stable release cycle after Phase A ships.
  // Order matters: specific sources must precede the /runtime/:path* and /protocol/:path* wildcards.
  async redirects() {
    return [
      {
        source: "/runtime/core/network/protocol",
        destination: "/docs/network/protocol-model/",
        statusCode: 301,
      },
      { source: "/runtime/cli-reference", destination: "/docs/cli/", statusCode: 301 },
      {
        source: "/runtime/cli-reference/:path*",
        destination: "/docs/cli/:path*/",
        statusCode: 301,
      },
      { source: "/runtime/api-reference", destination: "/docs/api/", statusCode: 301 },
      {
        source: "/runtime/api-reference/:path*",
        destination: "/docs/api/:path*/",
        statusCode: 301,
      },
      { source: "/runtime/core/:path*", destination: "/docs/:path*/", statusCode: 301 },
      { source: "/runtime", destination: "/docs/", statusCode: 301 },
      { source: "/runtime/:path*", destination: "/docs/:path*/", statusCode: 301 },
      { source: "/protocol", destination: "/docs/network/protocol/", statusCode: 301 },
      {
        source: "/protocol/:path*",
        destination: "/docs/network/protocol/:path*/",
        statusCode: 301,
      },
    ];
  },
};

export default withMDX(config);
