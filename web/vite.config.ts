import { proto } from "@compozy/proto/vite";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import { defineConfig } from "vite";

import { resolveApiProxyOrigin, resolveApiProxyTarget } from "./src/lib/vite-api-proxy-target.ts";
import presentationConfig from "./vite.presentation.ts";

const apiProxyTarget = resolveApiProxyTarget(process.env);
const apiProxyOrigin = resolveApiProxyOrigin(process.env);

export default defineConfig(({ command }) => ({
  plugins: [
    command === "serve" && devtools(),
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    ...(presentationConfig.plugins ?? []),
    // Prototypes, drafts, flows and the gallery never reach the SPA bundle. `proto dev` and
    // `proto build` reuse this config for the render origin, where the boundary steps aside.
    proto(),
  ],
  resolve: presentationConfig.resolve,
  build: presentationConfig.build,
  server: {
    host: "127.0.0.1",
    proxy: {
      "/api": {
        target: apiProxyTarget,
        changeOrigin: true,
        // Forward window-manager and document-wide event streams to the daemon.
        ws: true,
        headers: {
          Origin: apiProxyOrigin,
        },
      },
    },
  },
}));
