import tailwindcss from "@tailwindcss/vite";
import { tanstackStart } from "@tanstack/react-start/plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

import { visualExportIds } from "./src/manual/art/visuals.ts";

// Static manual: every route prerenders to .output/public. crawlLinks follows the chapter nav,
// failOnError keeps a broken page from shipping silently (ADR-001).
export default defineConfig({
  server: { port: 4620, strictPort: true, host: "127.0.0.1" },
  preview: { port: 4620, strictPort: true, host: "127.0.0.1" },
  resolve: { tsconfigPaths: true },
  // Static output layout the turbo cache and the build-output test read (_dx.md, IT-001).
  environments: {
    client: { build: { outDir: ".output/public" } },
    ssr: { build: { outDir: ".output/server" } },
  },
  plugins: [
    tailwindcss(),
    tanstackStart({
      prerender: {
        enabled: true,
        crawlLinks: true,
        failOnError: true,
        // Downloads under /brand/ are static files, not pages.
        filter: ({ path }) => !path.startsWith("/brand/"),
      },
      pages: [
        { path: "/404", prerender: { enabled: true, outputPath: "/404.html" } },
        // Export surfaces are not linked from the manual, so crawlLinks never reaches them.
        ...visualExportIds().map(id => ({ path: `/visual/${id}` })),
      ],
    }),
    viteReact(),
  ],
});
