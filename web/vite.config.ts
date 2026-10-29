import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import { defineConfig, type Plugin } from "vite";

import { resolveApiProxyOrigin, resolveApiProxyTarget } from "./src/lib/vite-api-proxy-target.ts";
import { presentationPlugins, webSourceRoot } from "./vite.presentation.ts";

const reactRuntimePattern =
  /[\\/]node_modules[\\/](?:\.bun[\\/][^\\/]+[\\/]node_modules[\\/])?(?:react|react-dom|scheduler|use-sync-external-store)[\\/]/;
const fontAssetPattern = /\.(?:eot|otf|ttf|woff2?)$/iu;

/** Prototype-only modules: `*.proto.*`, drafts, and the gallery harness with its flows. */
const prototypeModulePattern =
  /\.proto\.[cm]?[jt]sx?(?:\?|$)|[\\/]web[\\/](?:src[\\/]drafts|gallery)[\\/]/;

/** Fails the production build when a prototype-only module reaches a chunk. */
function productionBoundary(): Plugin {
  return {
    name: "compozy:production-boundary",
    generateBundle(_options, bundle) {
      for (const output of Object.values(bundle)) {
        if (output.type !== "chunk") continue;
        for (const id of Object.keys(output.modules)) {
          if (prototypeModulePattern.test(id)) {
            this.error(`Prototype-only module reached the production bundle: ${id}`);
          }
        }
      }
    },
  };
}

const apiProxyTarget = resolveApiProxyTarget(process.env);
const apiProxyOrigin = resolveApiProxyOrigin(process.env);

export default defineConfig(({ command }) => ({
  plugins: [
    command === "serve" && devtools(),
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    ...presentationPlugins(),
    productionBoundary(),
  ],
  resolve: {
    alias: {
      "@": webSourceRoot,
    },
  },
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
  build: {
    assetsInlineLimit: filePath => (fontAssetPattern.test(filePath) ? false : undefined),
    rolldownOptions: {
      output: {
        codeSplitting: {
          groups: [
            {
              name: "react-runtime",
              test: reactRuntimePattern,
              priority: 10,
            },
          ],
        },
      },
    },
  },
}));
