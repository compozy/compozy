import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact, { reactCompilerPreset } from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

import { resolveApiProxyOrigin, resolveApiProxyTarget } from "./src/lib/vite-api-proxy-target";

const reactRuntimePattern =
  /[\\/]node_modules[\\/](?:\.bun[\\/][^\\/]+[\\/]node_modules[\\/])?(?:react|react-dom|scheduler|use-sync-external-store)[\\/]/;

const apiProxyTarget = resolveApiProxyTarget(process.env);
const apiProxyOrigin = resolveApiProxyOrigin(process.env);

export default defineConfig(({ command }) => ({
  plugins: [
    command === "serve" && devtools(),
    tanstackRouter({
      target: "react",
      autoCodeSplitting: true,
    }),
    viteReact(),
    babel({
      presets: [reactCompilerPreset()],
    }),
    tailwindcss(),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: apiProxyTarget,
        changeOrigin: true,
        // Window-manager stream: forward WebSocket upgrades to the daemon.
        ws: true,
        headers: {
          Origin: apiProxyOrigin,
        },
      },
    },
  },
  build: {
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
