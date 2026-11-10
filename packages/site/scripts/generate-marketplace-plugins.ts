/**
 * Refreshes the public marketplace's plugin snapshot from the preset plugin marketplaces the daemon
 * enables by default (`catalog/v3/marketplaces.json`, `default: "on"`).
 *
 * Run by hand: `bun run generate:marketplace-plugins`. It is not part of `generate`, prebuild or CI;
 * `next build` only reads the checked-in result, so builds stay offline and deterministic.
 *
 * Decoding mirrors `internal/marketplace/pluginsource/document.go` (document locations, name pattern,
 * duplicate names, supported sources, author/x-compozy shapes) and the icon rule in
 * `internal/marketplace/entry_icon.go`. Category, featured and supersede projection happen in
 * `lib/marketplace-view.ts`, which reads this snapshot.
 */
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import type { PluginSnapshot, SnapshotPlugin } from "../lib/marketplace-plugins-schema";
import { pluginSnapshotSchema } from "../lib/marketplace-plugins-schema";
import { iconHints, type IconHints } from "./marketplace-icon-hints";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repoRoot = resolve(siteRoot, "..", "..");
const presetsPath = resolve(repoRoot, "catalog", "v3", "marketplaces.json");
const extensionsPath = resolve(repoRoot, "catalog", "v3", "extensions.json");
const snapshotPath = resolve(siteRoot, "lib", "marketplace-plugins.snapshot.json");
const iconRoot = resolve(siteRoot, "public", "catalog-icons");
/** Same order as `pluginsource.ReadDirectory`. */
const DOCUMENT_PATHS = [
  "marketplace.json",
  ".claude-plugin/marketplace.json",
  ".agents/plugins/marketplace.json",
];
const PLUGIN_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const MAX_ICON_BYTES = 64 * 1024;
const ICON_TYPES: Record<string, string> = {
  "image/png": "png",
  "image/svg+xml": "svg",
  "image/webp": "webp",
};

type Json = Record<string, unknown>;

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function texts(value: unknown): string[] {
  return Array.isArray(value) && value.every(item => typeof item === "string") ? value : [];
}

function authorName(value: unknown): string {
  if (typeof value === "string") return value;
  return value && typeof value === "object" ? text((value as Json).name) : "";
}

/** `github:owner/repo` and `https://github.com/owner/repo` presets → owner/repo. */
function githubRepository(source: string): string {
  const match = /^(?:github:|https:\/\/github\.com\/)([^/\s]+\/[^/\s]+?)(?:\.git)?\/?$/.exec(
    source
  );
  if (!match) throw new Error(`unsupported preset source ${source}`);
  return match[1];
}

function publicHttps(value: string): boolean {
  try {
    const url = new URL(value);
    return (
      url.protocol === "https:" &&
      !!url.hostname &&
      !url.username &&
      !url.password &&
      !url.search &&
      !url.hash
    );
  } catch {
    return false;
  }
}

/** The plugin sources `decodePluginSource` accepts; anything else is dropped as unsupported. */
function supportedSource(source: unknown): boolean {
  if (typeof source === "string" && source !== "") {
    if (source.startsWith("https://")) return publicHttps(source);
    return (
      !source.startsWith("/") &&
      !/[\\\0]/.test(source) &&
      !/^[a-z][a-z0-9+.-]*:/i.test(source) &&
      !source.split("/").includes("..")
    );
  }
  if (!source || typeof source !== "object") return false;
  const value = source as Json;
  switch (value.source) {
    case "url":
      return publicHttps(text(value.url));
    case "git-subdir": {
      const path = text(value.path);
      return (
        publicHttps(text(value.url)) &&
        path.trim() !== "" &&
        !path.startsWith("/") &&
        !path.split("/").includes("..")
      );
    }
    case "github":
      return /^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(text(value.repo).replace(/\.git$/, ""));
    default:
      return false;
  }
}

interface WrittenIcon extends IconHints {
  path: string;
}

/**
 * `ValidateIcon`: HTTPS PNG/SVG/WebP or a data URL of those types, at most 64 KiB; otherwise no icon.
 * The bytes land under `public/catalog-icons/` with the tile hints read from them.
 */
async function writeIcon(
  icon: string,
  source: string,
  name: string
): Promise<WrittenIcon | undefined> {
  if (!icon || icon.length > MAX_ICON_BYTES) return undefined;
  const match = /^data:(image\/(?:png|svg\+xml|webp))(;base64)?,(.+)$/s.exec(icon);
  if (!match) return undefined;
  const bytes = match[2]
    ? Buffer.from(match[3], "base64")
    : Buffer.from(decodeURIComponent(match[3]), "utf8");
  if (bytes.length === 0) return undefined;
  const file = `${source}/${name}.${ICON_TYPES[match[1]]}`;
  mkdirSync(resolve(iconRoot, source), { recursive: true });
  writeFileSync(resolve(iconRoot, file), bytes);
  return { path: `/catalog-icons/${file}`, ...(await iconHints(bytes, match[1])) };
}

const ICON_MEDIA: Record<string, string> = {
  ".png": "image/png",
  ".svg": "image/svg+xml",
  ".webp": "image/webp",
};

/**
 * The curated extensions' icons are published from `catalog/icons/`, so their hints come from the
 * same files in this checkout; an icon outside that folder gets no hints.
 */
async function extensionIconHints(): Promise<Record<string, IconHints>> {
  const feed = JSON.parse(readFileSync(extensionsPath, "utf8")) as { entries: Json[] };
  const hints: Record<string, IconHints> = {};
  for (const entry of feed.entries) {
    const icon = text(entry.icon);
    const match = /\/catalog\/icons\/([A-Za-z0-9._-]+(\.png|\.svg|\.webp))$/.exec(icon);
    const file = match ? resolve(repoRoot, "catalog", "icons", match[1]) : "";
    if (!match || !existsSync(file)) continue;
    hints[text(entry.entry_id)] = await iconHints(readFileSync(file), ICON_MEDIA[match[2]]);
  }
  return hints;
}

async function fetchDocument(repository: string) {
  for (const path of DOCUMENT_PATHS) {
    const response = await fetch(`https://raw.githubusercontent.com/${repository}/HEAD/${path}`, {
      signal: AbortSignal.timeout(30_000),
    });
    if (response.status === 404) continue;
    if (!response.ok) throw new Error(`${repository}/${path}: HTTP ${response.status}`);
    const raw = await response.text();
    return { path, raw, digest: createHash("sha256").update(raw).digest("hex") };
  }
  throw new Error(`${repository}: no marketplace document at ${DOCUMENT_PATHS.join(", ")}`);
}

async function decodePlugins(
  raw: string,
  source: string,
  curated: boolean
): Promise<SnapshotPlugin[]> {
  const plugins = (JSON.parse(raw) as Json).plugins;
  if (!Array.isArray(plugins)) throw new Error(`${source}: plugins must be an array`);
  const seen = new Set<string>();
  const decoded: SnapshotPlugin[] = [];
  for (const item of plugins as Json[]) {
    const name = text(item?.name);
    if (!PLUGIN_NAME.test(name) || seen.has(name) || !supportedSource(item.source)) continue;
    seen.add(name);
    const curation =
      item["x-compozy"] && typeof item["x-compozy"] === "object" ? (item["x-compozy"] as Json) : {};
    // Remote icon URLs are not allowed by next.config for plugins: they fall back to the monogram.
    const icon = await writeIcon(text(item.icon), source, name);
    if (text(item.icon).startsWith("https://"))
      console.warn(`${source}/${name}: remote icon left out`);
    decoded.push(
      Object.fromEntries(
        Object.entries({
          source,
          name,
          display_name: text(item.displayName).trim(),
          description: text(item.description),
          author: authorName(item.author).trim(),
          homepage: text(item.homepage),
          repository: text(item.repository).trim(),
          license: text(item.license),
          version: text(item.version),
          category: text(item.category),
          compozy_category: curated ? text(curation.category) : "",
          featured: curated && curation.featured === true,
          supersedes: curated ? texts(curation.supersedes) : [],
          icon: icon?.path,
          bleed: icon?.bleed === true,
          light: icon?.light === true,
        }).filter(
          ([, value]) =>
            value !== "" &&
            value !== false &&
            value !== undefined &&
            !(Array.isArray(value) && value.length === 0)
        )
      ) as SnapshotPlugin
    );
  }
  return decoded;
}

const presets = (
  JSON.parse(readFileSync(presetsPath, "utf8")) as { entries: Json[] }
).entries.filter(preset => preset.default === "on");
rmSync(iconRoot, { recursive: true, force: true });
const snapshot: PluginSnapshot = {
  snapshot_version: 1,
  sources: [],
  plugins: [],
  extension_icons: await extensionIconHints(),
};
for (const preset of presets) {
  const name = text(preset.name);
  const repository = githubRepository(text(preset.source));
  const document = await fetchDocument(repository);
  // `curatedIdentity`: only the CompozyOS plugin source carries x-compozy curation.
  const curated = name === "compozy-plugins" && repository === "compozy/plugins";
  const plugins = (await decodePlugins(document.raw, name, curated)).toSorted((a, b) =>
    a.name.localeCompare(b.name)
  );
  snapshot.sources.push({
    name,
    repository,
    document_path: document.path,
    digest_sha256: document.digest,
    plugins: plugins.length,
  });
  snapshot.plugins.push(...plugins);
}
pluginSnapshotSchema.parse(snapshot);
writeFileSync(snapshotPath, `${JSON.stringify(snapshot, null, 2)}\n`, "utf8");
// The repository formatter owns JSON layout; format here so a refresh only diffs real changes.
const formatted = spawnSync("bunx", ["oxfmt", snapshotPath], { cwd: repoRoot, stdio: "inherit" });
if (formatted.status !== 0) throw new Error(`oxfmt failed on ${snapshotPath}`);
console.log(
  `generated ${snapshotPath}: ${snapshot.sources.map(source => `${source.name} ${source.plugins}`).join(", ")}`
);
