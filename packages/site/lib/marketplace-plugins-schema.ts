/**
 * The checked-in plugin snapshot written by `scripts/generate-marketplace-plugins.ts`: each preset
 * plugin marketplace's decoded plugins, as `pluginsource.DecodeDocument` reads them. Projection
 * (category, featured, supersedes) is not stored; `lib/marketplace-view.ts` applies it.
 */
import { z } from "zod";

const PLUGIN_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const SOURCE_NAME = /^[a-z0-9._-]{1,64}$/;

const pluginSchema = z.strictObject({
  source: z.string().regex(SOURCE_NAME),
  name: z.string().regex(PLUGIN_NAME),
  /** The feed's `displayName`; the daemon ignores it, the public page shows it. */
  display_name: z.string().optional(),
  description: z.string().optional(),
  author: z.string().optional(),
  homepage: z.string().optional(),
  repository: z.string().optional(),
  license: z.string().optional(),
  version: z.string().optional(),
  /** The publisher's own category, resolved through the daemon's alias table. */
  category: z.string().optional(),
  /** `x-compozy` curation, kept only for the curated CompozyOS source. */
  compozy_category: z.string().optional(),
  featured: z.literal(true).optional(),
  supersedes: z.array(z.string().regex(/^[^/]+\/[^/]+$/)).optional(),
  /** Site path of the icon the feed carried inline, extracted under `public/catalog-icons/`. */
  icon: z
    .string()
    .regex(/^\/catalog-icons\/[a-z0-9._-]+\/[A-Za-z0-9._-]+\.(?:png|svg|webp)$/)
    .optional(),
  /** Tile hints read from the icon bytes (`scripts/marketplace-icon-hints.ts`). */
  bleed: z.literal(true).optional(),
  light: z.literal(true).optional(),
});

const iconHintsSchema = z.strictObject({ bleed: z.boolean(), light: z.boolean() });

export const pluginSnapshotSchema = z
  .strictObject({
    snapshot_version: z.literal(1),
    sources: z.array(
      z.strictObject({
        name: z.string().regex(SOURCE_NAME),
        repository: z.string().min(1),
        document_path: z.string().min(1),
        digest_sha256: z.string().regex(/^[a-f0-9]{64}$/),
        plugins: z.number().int().nonnegative(),
      })
    ),
    plugins: z.array(pluginSchema),
    /** Tile hints for the curated extensions' icons, by entry id, from `catalog/icons/`. */
    extension_icons: z.record(z.string(), iconHintsSchema),
  })
  .superRefine((snapshot, ctx) => {
    const seen = new Set<string>();
    const sources = new Set(snapshot.sources.map(source => source.name));
    for (const [index, plugin] of snapshot.plugins.entries()) {
      const key = `${plugin.source}/${plugin.name}`;
      if (seen.has(key))
        ctx.addIssue({ code: "custom", message: "plugin is duplicated", path: ["plugins", index] });
      if (!sources.has(plugin.source))
        ctx.addIssue({
          code: "custom",
          message: "unknown source",
          path: ["plugins", index, "source"],
        });
      seen.add(key);
    }
  });

export type PluginSnapshot = z.infer<typeof pluginSnapshotSchema>;
export type SnapshotPlugin = PluginSnapshot["plugins"][number];
