import { execFileSync } from "node:child_process";
import { resolve } from "node:path";
import { defineConfig } from "vite";

import { presentationPlugins, webSourceRoot } from "../vite.presentation.ts";
import { commentsApi } from "./node/plugins/comments.ts";
import { networkGuard } from "./node/plugins/network-guard.ts";
import { prototypeRegistry } from "./node/plugins/prototypes.ts";
import { galleryHost, galleryPort, galleryRoot, repoRoot, webRoot } from "./node/paths.ts";
import { sourceTags } from "./node/source-tags.ts";

/** The checkout's commit, recorded in each new review comment (`context.commit`). */
function headCommit() {
  try {
    return execFileSync("git", ["rev-parse", "HEAD"], { cwd: repoRoot, encoding: "utf8" }).trim();
  } catch {
    return null;
  }
}

/**
 * The prototype gallery: the app's rendering pieces (React Compiler, Tailwind, `@`) without its
 * route generation, daemon proxy or production boundary. Prototypes load lazily through the
 * virtual registry, which the dependency scanner cannot follow, so it scans them explicitly.
 */
export default defineConfig({
  root: galleryRoot,
  cacheDir: resolve(webRoot, "node_modules/.cache/gallery"),
  publicDir: resolve(webRoot, "public"),
  optimizeDeps: { entries: ["index.html", "../src/**/*.proto.tsx"] },
  server: { host: galleryHost, port: galleryPort, strictPort: true },
  preview: { host: galleryHost, port: galleryPort, strictPort: true },
  build: { outDir: resolve(webRoot, "dist-gallery"), emptyOutDir: true },
  resolve: { alias: { "@": webSourceRoot, "#gallery": galleryRoot } },
  define: { "import.meta.env.VITE_PROTO_COMMIT": JSON.stringify(headCommit()) },
  plugins: [
    sourceTags(repoRoot),
    prototypeRegistry(webRoot, repoRoot),
    ...presentationPlugins(),
    networkGuard(webRoot),
    commentsApi(),
  ],
});
