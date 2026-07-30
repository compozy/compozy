import { readdirSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { parse as parseToml } from "smol-toml";
import { z } from "zod";

/**
 * Bridge providers are not catalog entries and cannot be today: each `extension.toml` points its
 * `[subprocess]` command at a locally built `./bin/<platform>` binary, so there is no single
 * cross-platform artifact to publish with a digest. The honest source is the manifest set in the
 * repository, read at build time — which also means this list can never claim a provider the repo
 * does not ship.
 *
 * The schema mirrors what `internal/extension` requires of a bridge manifest; a manifest the daemon
 * would reject fails this parse, and the site build with it.
 */

const bridgesRoot = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "..",
  "extensions",
  "bridges"
);

const bridgeManifestSchema = z.object({
  extension: z.object({
    name: z.string().min(1),
    version: z.string().min(1),
    description: z.string().min(1),
    min_compozy_version: z.string().min(1).optional(),
  }),
  capabilities: z.object({
    provides: z.array(z.string().min(1)).refine(values => values.includes("bridge.adapter"), {
      message: "a bridge provider must provide bridge.adapter",
    }),
  }),
  bridge: z.object({
    platform: z.string().min(1),
    display_name: z.string().min(1),
    secret_slots: z
      .array(
        z.object({
          name: z.string().min(1),
          description: z.string().min(1),
          required: z.boolean(),
        })
      )
      .min(1),
  }),
});

export interface BridgeProvider {
  platform: string;
  displayName: string;
  version: string;
  description: string;
  secretSlots: { required: number; total: number };
  /** Setup guide inside the docs tree; every in-tree provider has one. */
  setupUrl: string;
}

/**
 * Display order, matching the landing page's bridges section
 * (`components/landing/bridges-section.tsx`) so the same eight providers read in the same sequence
 * on both surfaces. A provider missing from this list sorts last, alphabetically.
 */
const BRIDGE_DISPLAY_ORDER = [
  "slack",
  "discord",
  "telegram",
  "whatsapp",
  "teams",
  "gchat",
  "github",
  "linear",
];

function displayRank(platform: string): number {
  const index = BRIDGE_DISPLAY_ORDER.indexOf(platform);
  return index === -1 ? BRIDGE_DISPLAY_ORDER.length : index;
}

function readBridgeProviders(): BridgeProvider[] {
  const directories: string[] = [];
  for (const entry of readdirSync(bridgesRoot, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      directories.push(entry.name);
    }
  }
  directories.sort();

  const providers: BridgeProvider[] = [];
  for (const directory of directories) {
    const manifestPath = resolve(bridgesRoot, directory, "extension.toml");
    let raw: string;
    try {
      raw = readFileSync(manifestPath, "utf8");
    } catch {
      // A directory without a manifest is not a provider (shared helpers, fixtures).
      continue;
    }
    const manifest = bridgeManifestSchema.parse(parseToml(raw));
    const slots = manifest.bridge.secret_slots;
    providers.push({
      platform: manifest.bridge.platform,
      displayName: manifest.bridge.display_name,
      version: manifest.extension.version,
      description: manifest.extension.description,
      secretSlots: {
        required: slots.filter(slot => slot.required).length,
        total: slots.length,
      },
      setupUrl: `/docs/bridges/setup-${manifest.bridge.platform}`,
    });
  }
  providers.sort((left, right) => {
    const byRank = displayRank(left.platform) - displayRank(right.platform);
    return byRank === 0 ? left.platform.localeCompare(right.platform) : byRank;
  });
  return providers;
}

export const bridgeProviders: BridgeProvider[] = readBridgeProviders();

export function findBridgeProvider(platform: string): BridgeProvider | undefined {
  return bridgeProviders.find(provider => provider.platform === platform);
}
