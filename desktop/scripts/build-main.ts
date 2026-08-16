import { cp, mkdir, rm } from "node:fs/promises";
import { join } from "node:path";

import { build } from "esbuild";

const root = join(import.meta.dir, "..");
const output = join(root, "dist");
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });

await Promise.all([
  build({
    entryPoints: [join(root, "src", "main.ts")],
    outfile: join(output, "main.cjs"),
    bundle: true,
    platform: "node",
    format: "cjs",
    target: "node22",
    external: ["electron", "electron-updater"],
    sourcemap: true,
  }),
  build({
    entryPoints: [join(root, "src", "boot", "boot-preload.ts")],
    outfile: join(output, "boot-preload.cjs"),
    bundle: true,
    platform: "node",
    format: "cjs",
    target: "node22",
    external: ["electron"],
    sourcemap: true,
  }),
  cp(join(root, "pages"), join(output, "pages"), { recursive: true }),
]);
