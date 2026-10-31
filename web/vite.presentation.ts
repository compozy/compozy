import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import viteReact, { reactCompilerPreset } from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";

const reactRuntimePattern =
  /[\\/]node_modules[\\/](?:\.bun[\\/][^\\/]+[\\/]node_modules[\\/])?(?:react|react-dom|scheduler|use-sync-external-store)[\\/]/;
const fontAssetPattern = /\.(?:eot|otf|ttf|woff2?)$/iu;

/**
 * Rendering and bundling pieces the app, Storybook and the prototype gallery share: React with
 * the React Compiler, Tailwind, the `@` alias and chunking. `vite.config.ts` adds routing, the
 * daemon proxy and the `proto()` production boundary; Storybook builds from this file alone,
 * because that boundary treats stories as prototype-only modules.
 */
export default defineConfig({
  plugins: [
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
});
