import babel from "@rolldown/plugin-babel";
import tailwindcss from "@tailwindcss/vite";
import viteReact, { reactCompilerPreset } from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";
import type { PluginOption } from "vite";

/**
 * Rendering pieces the app and the prototype gallery share: React with the
 * React Compiler, Tailwind, and the `@` alias. The app adds routing, the daemon
 * proxy and the production boundary; the gallery adds prototype discovery.
 */
export const webSourceRoot = fileURLToPath(new URL("./src", import.meta.url));

export function presentationPlugins(): PluginOption[] {
  return [
    viteReact(),
    babel({
      presets: [reactCompilerPreset()],
    }),
    tailwindcss(),
  ];
}
