import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { build, type BuildOptions } from "esbuild";

import { resolveDesktopBuildChannel } from "../src/release/build-channel";

const root = join(import.meta.dir, "..");
const output = join(root, "dist");
const e2eBuild = process.env.COMPOZY_DESKTOP_E2E_BUILD === "1";
const releaseChannel = resolveDesktopBuildChannel(process.env.COMPOZY_RELEASE_CHANNEL);
await rm(output, { recursive: true, force: true });
await mkdir(output, { recursive: true });
await cp(join(root, "pages"), join(output, "pages"), { recursive: true });

const canonicalTokens = await readFile(
  join(root, "..", "packages", "ui", "src", "tokens.css"),
  "utf8"
);
const bootTokenMap = {
  canvas: "color-canvas",
  fg: "color-fg",
  strong: "color-fg-strong",
  muted: "color-subtle",
  action: "color-accent",
  "action-hover": "color-accent-hover",
  success: "color-success",
  danger: "color-danger",
  warning: "color-warning",
  info: "color-info",
  line: "color-line",
  surface: "color-surface-glaze",
  "surface-hover": "color-btn-default-hover",
  sans: "font-sans",
  mono: "font-mono",
} as const;
const bootTokens = Object.entries(bootTokenMap).map(([bootName, canonicalName]) => {
  const match = new RegExp(`^\\s*--${canonicalName}:\\s*(.+);$`, "mu").exec(canonicalTokens);
  if (!match?.[1]) throw new Error(`Canonical desktop token --${canonicalName} is missing.`);
  return `  --${bootName}: ${match[1]};`;
});
await writeFile(
  join(output, "pages", "boot-tokens.css"),
  `/* Generated from packages/ui/src/tokens.css by build-main.ts. */\n:root {\n  color-scheme: dark;\n${bootTokens.join("\n")}\n}\n`,
  "utf8"
);

const sharedOptions = {
  bundle: true,
  platform: "node",
  format: "cjs",
  target: "node22",
  sourcemap: true,
} satisfies BuildOptions;

await Promise.all([
  build({
    ...sharedOptions,
    entryPoints: [join(root, "src", "main.ts")],
    outfile: join(output, "main.cjs"),
    external: ["electron", "electron-updater"],
    define: {
      __COMPOZY_DESKTOP_E2E_BUILD__: JSON.stringify(e2eBuild),
      __COMPOZY_RELEASE_CHANNEL__: JSON.stringify(releaseChannel),
    },
  }),
  build({
    ...sharedOptions,
    entryPoints: [join(root, "src", "boot", "boot-preload.ts")],
    outfile: join(output, "boot-preload.cjs"),
    external: ["electron"],
  }),
  build({
    entryPoints: [join(root, "src", "state", "public-safe-text.ts")],
    outfile: join(output, "pages", "public-safe-text.js"),
    bundle: true,
    platform: "browser",
    format: "esm",
    target: "es2023",
    sourcemap: true,
  }),
]);
