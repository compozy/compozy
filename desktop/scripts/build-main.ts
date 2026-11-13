import { cp, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { build, type BuildOptions } from "esbuild";

import { resolveDesktopBuildChannel } from "../src/release/build-channel";
import { writeBootAssets } from "./boot-assets";

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
// Boot page aliases → canonical @compozy/ui tokens (approved prototype: web/src/drafts/desktop-boot).
const bootTokenMap = {
  ink: "color-rail",
  panel: "color-canvas",
  surface: "color-surface-2",
  line: "color-line",
  "line-strong": "color-line-strong",
  point: "color-line",
  "point-done": "color-neutral",
  fg: "color-fg",
  strong: "color-fg-strong",
  "fg-2": "color-fg-2",
  muted: "color-muted-foreground",
  primary: "color-primary",
  "primary-hover": "color-primary-hover",
  "primary-ink": "color-primary-foreground",
  "brand-text": "color-brand-strong",
  // Volt-to-cyan is the live signal on ink; paper keeps the brand blue (tokens-light re-declares it).
  live: "color-brand-cyan",
  selection: "color-brand-tint",
  "focus-ring": "theme-shadow-focus-ring",
  success: "color-success",
  danger: "color-destructive",
  warning: "color-warning",
  sans: "font-sans",
  mono: "font-mono",
  display: "font-display",
  ease: "ease-out",
} as const;
/** Light-theme overrides for aliases whose light value comes from another canonical token. */
const bootLightOverrides: Partial<Record<keyof typeof bootTokenMap, string>> = {
  live: "color-brand",
  point: "color-line-strong",
};
const lightTokens = await readFile(
  join(root, "..", "packages", "ui", "src", "tokens-light.css"),
  "utf8"
);
/** A declaration's value, including values wrapped across lines (font stacks). */
function tokenValue(source: string, name: string): string | undefined {
  const value = new RegExp(`^\\s*--${name}:\\s*([^;]+);`, "mu").exec(source)?.[1];
  // The boot page renders before the daemon is reachable, so it cannot know
  // the operator's Appearance font: drop the `--user-font-*` head the web
  // runtime fills (it is undefined here and would void the whole stack).
  return value
    ?.replace(/\s+/gu, " ")
    .trim()
    .replace(/^var\(--user-font-[a-z]+\),\s*/u, "");
}
const bootTokens = Object.entries(bootTokenMap).map(([bootName, canonicalName]) => {
  const value = tokenValue(canonicalTokens, canonicalName);
  if (!value) throw new Error(`Canonical desktop token --${canonicalName} is missing.`);
  return `  --${bootName}: ${value};`;
});
// The boot page follows the user's theme through prefers-color-scheme: main sets
// nativeTheme.themeSource to the stored preference before the boot window exists.
// Tokens the light theme does not re-declare (fonts) keep their dark value.
const bootLightTokens = Object.entries(bootTokenMap).flatMap(([bootName, canonicalName]) => {
  const override = bootLightOverrides[bootName as keyof typeof bootTokenMap];
  const value = override
    ? (tokenValue(lightTokens, override) ?? tokenValue(canonicalTokens, override))
    : tokenValue(lightTokens, canonicalName);
  return value ? [`    --${bootName}: ${value};`] : [];
});
await writeFile(
  join(output, "pages", "boot-tokens.css"),
  `/* Generated from packages/ui/src/tokens.css and tokens-light.css by build-main.ts. */\n` +
    `:root {\n  color-scheme: dark;\n${bootTokens.join("\n")}\n` +
    `  --ease-loop: cubic-bezier(0.45, 0, 0.2, 1);\n}\n` +
    `@media (prefers-color-scheme: light) {\n  :root {\n    color-scheme: light;\n` +
    `${bootLightTokens.join("\n")}\n  }\n}\n`,
  "utf8"
);

const sharedOptions = {
  bundle: true,
  platform: "node",
  format: "cjs",
  target: "node22",
  sourcemap: true,
} satisfies BuildOptions;

await writeBootAssets(root, join(output, "pages"));

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
    ...sharedOptions,
    entryPoints: [join(root, "src", "product", "product-preload.ts")],
    outfile: join(output, "product-preload.cjs"),
    external: ["electron"],
  }),
  build({
    entryPoints: [join(root, "src", "boot", "page", "main.ts")],
    outfile: join(output, "pages", "boot.js"),
    bundle: true,
    platform: "browser",
    format: "esm",
    target: "es2023",
    sourcemap: true,
  }),
]);
