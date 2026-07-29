import { z } from "zod";
import extensionsFeed from "../../../catalog/extensions.json";
import mcpFeed from "../../../catalog/mcp.json";
import skillsFeed from "../../../catalog/skills.json";

/**
 * Build-time mirror of the daemon feed contract in `internal/marketplace/entry_*.go`. A field
 * change in `catalog/*.json` that the daemon would reject fails this parse — and the site build
 * with it (site co-ship rule, spec §7.1).
 */

const ENTRY_ID_PATTERN = /^[A-Za-z0-9._~-]+$/;
const SHA256_PATTERN = /^[a-f0-9]{64}$/i;
const RFC3339 = z.string().refine(value => !Number.isNaN(Date.parse(value)), {
  message: "must be an RFC3339 timestamp",
});

const entryCommon = {
  entry_id: z.string().regex(ENTRY_ID_PATTERN, "entry_id must be one URL-safe path segment"),
  name: z.string().min(1),
  description: z.string().min(1),
  version: z.string().min(1).optional(),
  published_at: RFC3339.optional(),
  updated_at: RFC3339.optional(),
};

export const skillEntrySchema = z.strictObject({
  ...entryCommon,
  install_slug: z.string().min(1),
  display_name: z.string().min(1).optional(),
  author: z.string().min(1).optional(),
  tags: z.array(z.string().min(1)).optional(),
});

export const extensionEntrySchema = z.strictObject({
  ...entryCommon,
  version: z.string().min(1),
  install_slug: z.string().min(1),
  artifact_url: z
    .string()
    .url()
    .refine(value => value.startsWith("https://"), { message: "artifact_url must be HTTPS" }),
  digest_sha256: z.string().regex(SHA256_PATTERN, "digest_sha256 must be 64 hex characters"),
  tier: z.enum(["official", "community", "unverified"]),
  author: z.string().min(1).optional(),
  repository: z.string().url().optional(),
});

const mcpOAuthSchema = z.strictObject({
  issuer_url: z.string().url().optional(),
  authorization_url: z.string().url().optional(),
  token_url: z.string().url().optional(),
  client_id: z.string().min(1),
  scopes: z.array(z.string().min(1)).optional(),
});

const mcpEnvFieldSchema = z.strictObject({
  name: z.string().min(1),
  prompt: z.string().min(1).optional(),
  required: z.boolean(),
  secret: z.boolean(),
  default: z.string().optional(),
});

export const mcpEntrySchema = z
  .strictObject({
    ...entryCommon,
    transport: z.enum(["stdio", "http", "sse"]),
    command: z.string().min(1).optional(),
    args: z.array(z.string()).optional(),
    url: z.string().url().optional(),
    oauth: mcpOAuthSchema.optional(),
    env: z.array(mcpEnvFieldSchema).optional(),
    default_scope: z.enum(["workspace", "global"]).optional(),
  })
  .superRefine((entry, ctx) => {
    if (entry.transport === "stdio") {
      if (!entry.command) {
        ctx.addIssue({
          code: "custom",
          message: "command is required for stdio",
          path: ["command"],
        });
      }
      if (entry.url) {
        ctx.addIssue({ code: "custom", message: "url is not allowed for stdio", path: ["url"] });
      }
      if (entry.oauth) {
        ctx.addIssue({
          code: "custom",
          message: "oauth is not allowed for stdio",
          path: ["oauth"],
        });
      }
      return;
    }
    if (!entry.url) {
      ctx.addIssue({
        code: "custom",
        message: `url is required for ${entry.transport}`,
        path: ["url"],
      });
    }
    if (entry.command || (entry.args?.length ?? 0) > 0) {
      ctx.addIssue({
        code: "custom",
        message: `command/args are not allowed for ${entry.transport}`,
        path: ["command"],
      });
    }
    if ((entry.env?.length ?? 0) > 0) {
      ctx.addIssue({
        code: "custom",
        message: `env is not allowed for ${entry.transport}`,
        path: ["env"],
      });
    }
  });

function feedSchema<EntrySchema extends z.ZodType>(entry: EntrySchema) {
  return z.strictObject({
    manifest_version: z.literal(1),
    generated_at: RFC3339.optional(),
    entries: z.array(entry).min(1),
  });
}

export type SkillEntry = z.infer<typeof skillEntrySchema>;
export type ExtensionEntry = z.infer<typeof extensionEntrySchema>;
export type MCPEntry = z.infer<typeof mcpEntrySchema>;

export type MarketplaceKind = "skills" | "extensions" | "mcp";
export type MarketplaceEntry = SkillEntry | ExtensionEntry | MCPEntry;

export const skillEntries: SkillEntry[] = feedSchema(skillEntrySchema).parse(skillsFeed).entries;
export const extensionEntries: ExtensionEntry[] =
  feedSchema(extensionEntrySchema).parse(extensionsFeed).entries;
export const mcpEntries: MCPEntry[] = feedSchema(mcpEntrySchema).parse(mcpFeed).entries;

export const MARKETPLACE_KINDS: readonly MarketplaceKind[] = ["skills", "extensions", "mcp"];

export function isMarketplaceKind(value: string): value is MarketplaceKind {
  return (MARKETPLACE_KINDS as readonly string[]).includes(value);
}

export function entriesForKind(kind: MarketplaceKind): MarketplaceEntry[] {
  switch (kind) {
    case "skills":
      return skillEntries;
    case "extensions":
      return extensionEntries;
    case "mcp":
      return mcpEntries;
  }
}

export function findEntry(kind: MarketplaceKind, entryId: string): MarketplaceEntry | undefined {
  return entriesForKind(kind).find(entry => entry.entry_id === entryId);
}

/** The CTA is the command — the site has no daemon to install into (spec §7.3). */
export function installCommand(kind: MarketplaceKind, entry: MarketplaceEntry): string {
  switch (kind) {
    case "skills":
      return `compozy skill install ${(entry as SkillEntry).install_slug}`;
    case "extensions":
      return `compozy extension install ${(entry as ExtensionEntry).install_slug}`;
    case "mcp":
      return `compozy mcp install ${entry.entry_id}`;
  }
}
