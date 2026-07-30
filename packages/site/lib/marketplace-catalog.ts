import { z } from "zod";
import extensionsFeed from "../../../catalog/extensions.json";
import mcpFeed from "../../../catalog/mcp.json";
import skillsFeed from "../../../catalog/skills.json";

/**
 * Build-time validation mirror of `internal/marketplace`. This validates the checked-in catalog
 * snapshot rendered by the site; a running daemon can instead use its configured, active source.
 */

const ENTRY_ID_PATTERN = /^[A-Za-z0-9._~-]+$/;
const ENV_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;
const SHA256_PATTERN = /^[a-f0-9]{64}$/i;
const RFC3339_PATTERN =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const MAX_CATALOG_ENTRIES_PER_KIND = 50_000;

const trimmedString = z.string().transform(value => value.trim());
const nonBlankString = trimmedString.refine(value => value.length > 0, {
  message: "is required",
});

function isRFC3339(value: string): boolean {
  const match = RFC3339_PATTERN.exec(value);
  if (!match) return false;

  const [, year, month, day, hour, minute, second, timezone] = match;
  const numericYear = Number(year);
  const numericMonth = Number(month);
  const numericDay = Number(day);
  const numericHour = Number(hour);
  const numericMinute = Number(minute);
  const numericSecond = Number(second);
  const daysInMonth = new Date(Date.UTC(numericYear, numericMonth, 0)).getUTCDate();

  if (
    numericMonth < 1 ||
    numericMonth > 12 ||
    numericDay < 1 ||
    numericDay > daysInMonth ||
    numericHour > 23 ||
    numericMinute > 59 ||
    numericSecond > 59
  ) {
    return false;
  }
  if (timezone === "Z") return true;

  const timezoneMatch = /^([+-])(\d{2}):(\d{2})$/.exec(timezone);
  return timezoneMatch !== null && Number(timezoneMatch[2]) <= 23 && Number(timezoneMatch[3]) <= 59;
}

function parseAbsoluteURL(value: string): URL | null {
  try {
    const parsed = new URL(value);
    if (
      (parsed.protocol !== "http:" && parsed.protocol !== "https:") ||
      parsed.hostname === "" ||
      parsed.username !== "" ||
      parsed.password !== ""
    ) {
      return null;
    }
    return parsed;
  } catch {
    return null;
  }
}

function isLoopbackHost(hostname: string): boolean {
  const normalized = hostname.replace(/^\[|\]$/g, "").toLowerCase();
  if (normalized === "localhost") return true;

  const ipv4 = normalized.split(".");
  if (ipv4.length === 4 && ipv4.every(part => /^\d{1,3}$/.test(part) && Number(part) <= 255)) {
    return Number(ipv4[0]) === 127;
  }

  if (normalized === "::1" || normalized === "0:0:0:0:0:0:0:1") return true;
  for (const prefix of ["::ffff:", "0:0:0:0:0:ffff:"]) {
    if (normalized.startsWith(prefix)) {
      return isLoopbackHost(normalized.slice(prefix.length));
    }
  }
  return false;
}

function isMCPEnvironmentName(value: string, secret: boolean): boolean {
  const normalized = value.toUpperCase();
  if (!ENV_NAME_PATTERN.test(value)) return false;
  if (
    normalized === "NODE_OPTIONS" ||
    normalized === "PYTHONPATH" ||
    normalized === "PYTHONHOME" ||
    normalized === "LD_PRELOAD" ||
    normalized.startsWith("DYLD_")
  ) {
    return false;
  }
  if (secret || /_(URL|URI|PATH|FILE|DIR)$/.test(normalized)) return true;
  return !/(SECRET|TOKEN|PASSWORD|PASSWD|API_KEY|APIKEY|PRIVATE_KEY|PRIVATEKEY|AUTHORIZATION|BEARER|CREDENTIAL)/.test(
    normalized
  );
}

function isAbsoluteHTTPURL(value: string): boolean {
  return parseAbsoluteURL(value) !== null;
}

function isOAuthURL(value: string): boolean {
  const parsed = parseAbsoluteURL(value);
  if (!parsed) return false;
  return parsed.protocol === "https:" || isLoopbackHost(parsed.hostname);
}

const rfc3339 = trimmedString.refine(isRFC3339, { message: "must be an RFC3339 timestamp" });
const entryID = nonBlankString.refine(
  value => value !== "." && value !== ".." && ENTRY_ID_PATTERN.test(value),
  { message: "must be one URL-safe path segment" }
);

const entryCommon = {
  entry_id: entryID,
  name: nonBlankString,
  description: nonBlankString,
  version: trimmedString.optional(),
  published_at: rfc3339.optional(),
  updated_at: rfc3339.optional(),
};

export const skillEntrySchema = z.strictObject({
  ...entryCommon,
  install_slug: nonBlankString,
  display_name: trimmedString.optional(),
  author: trimmedString.optional(),
  tags: z.array(z.string()).optional(),
});

export const extensionEntrySchema = z
  .strictObject({
    ...entryCommon,
    version: nonBlankString,
    install_slug: nonBlankString,
    artifact_url: nonBlankString,
    digest_sha256: nonBlankString.refine(value => SHA256_PATTERN.test(value), {
      message: "must be 64 hex characters",
    }),
    tier: nonBlankString
      .transform(value => value.toLowerCase())
      .pipe(z.enum(["official", "community", "unverified"])),
    author: trimmedString.optional(),
    repository: trimmedString.optional(),
  })
  .superRefine((entry, ctx) => {
    const artifact = parseAbsoluteURL(entry.artifact_url);
    if (!artifact || artifact.protocol !== "https:" || artifact.hash !== "") {
      ctx.addIssue({
        code: "custom",
        message: "artifact_url must be an absolute HTTPS URL without credentials or a fragment",
        path: ["artifact_url"],
      });
    }
  });

const mcpOAuthSchema = z
  .strictObject({
    issuer_url: trimmedString.optional(),
    authorization_url: trimmedString.optional(),
    token_url: trimmedString.optional(),
    client_id: nonBlankString,
    scopes: z.array(z.string()).optional(),
  })
  .superRefine((oauth, ctx) => {
    if (!oauth.issuer_url && (!oauth.authorization_url || !oauth.token_url)) {
      ctx.addIssue({
        code: "custom",
        message: "oauth requires issuer_url or both authorization_url and token_url",
      });
    }
    for (const [key, value] of Object.entries({
      issuer_url: oauth.issuer_url,
      authorization_url: oauth.authorization_url,
      token_url: oauth.token_url,
    })) {
      if (value && !isOAuthURL(value)) {
        ctx.addIssue({
          code: "custom",
          message: `${key} must use HTTPS unless host is loopback`,
          path: [key],
        });
      }
    }
  });

const mcpEnvFieldSchema = z
  .strictObject({
    name: nonBlankString,
    prompt: trimmedString.optional(),
    required: z.boolean(),
    secret: z.boolean(),
    default: z.string().optional(),
  })
  .superRefine((field, ctx) => {
    if (!isMCPEnvironmentName(field.name, field.secret)) {
      ctx.addIssue({
        code: "custom",
        message: "name must be a permitted environment variable",
        path: ["name"],
      });
    }
    if (field.secret && field.default && field.default.trim() !== "") {
      ctx.addIssue({
        code: "custom",
        message: "secret fields must not set default",
        path: ["default"],
      });
    }
  });

export const mcpEntrySchema = z
  .strictObject({
    ...entryCommon,
    transport: trimmedString.pipe(z.enum(["stdio", "http", "sse"])),
    command: trimmedString.optional(),
    args: z.array(z.string()).optional(),
    url: trimmedString.optional(),
    oauth: mcpOAuthSchema.optional(),
    env: z.array(mcpEnvFieldSchema).optional(),
    default_scope: trimmedString.optional(),
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
      if (entry.url)
        ctx.addIssue({ code: "custom", message: "url is not allowed for stdio", path: ["url"] });
      if (entry.oauth)
        ctx.addIssue({
          code: "custom",
          message: "oauth is not allowed for stdio",
          path: ["oauth"],
        });
    } else {
      if (!entry.url) {
        ctx.addIssue({
          code: "custom",
          message: `url is required for ${entry.transport}`,
          path: ["url"],
        });
      } else if (!isAbsoluteHTTPURL(entry.url)) {
        ctx.addIssue({
          code: "custom",
          message: "url must be an absolute HTTP(S) URL",
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
    }
    const seen = new Set<string>();
    for (const [index, field] of (entry.env ?? []).entries()) {
      if (seen.has(field.name)) {
        ctx.addIssue({
          code: "custom",
          message: "env names must be unique",
          path: ["env", index, "name"],
        });
      }
      seen.add(field.name);
    }
    if (
      entry.default_scope &&
      entry.default_scope !== "workspace" &&
      entry.default_scope !== "global"
    ) {
      ctx.addIssue({
        code: "custom",
        message: "default_scope must be workspace or global",
        path: ["default_scope"],
      });
    }
  });

export type SkillEntry = z.infer<typeof skillEntrySchema>;
export type ExtensionEntry = z.infer<typeof extensionEntrySchema>;
export type MCPEntry = z.infer<typeof mcpEntrySchema>;

export type MarketplaceKind = "skills" | "extensions" | "mcp";
export type MarketplaceEntry = SkillEntry | ExtensionEntry | MCPEntry;

export const MARKETPLACE_KINDS: readonly MarketplaceKind[] = ["skills", "extensions", "mcp"];
export const MARKETPLACE_FEED_FILENAMES = ["skills.json", "extensions.json", "mcp.json"] as const;
export const MARKETPLACE_SEARCH_COMMAND = "compozy marketplace search";

type FeedEntry = { entry_id: string; install_slug?: string };

function catalogFeedSchema<Entry extends z.ZodType>(kind: MarketplaceKind, entry: Entry) {
  return z
    .strictObject({
      manifest_version: z.literal(1),
      generated_at: rfc3339,
      entries: z.array(entry).min(1).max(MAX_CATALOG_ENTRIES_PER_KIND),
    })
    .superRefine((feed, ctx) => {
      const ids = new Set<string>();
      const slugs = new Set<string>();
      for (const [index, rawEntry] of (feed.entries as FeedEntry[]).entries()) {
        if (kind === "skills" && rawEntry.entry_id.startsWith("skill_")) {
          ctx.addIssue({
            code: "custom",
            message: "entry_id uses a reserved prefix",
            path: ["entries", index, "entry_id"],
          });
        }
        if (ids.has(rawEntry.entry_id)) {
          ctx.addIssue({
            code: "custom",
            message: "entry_id is duplicated",
            path: ["entries", index, "entry_id"],
          });
        }
        ids.add(rawEntry.entry_id);
        if (kind === "mcp") continue;
        const slug = rawEntry.install_slug ?? "";
        if (slugs.has(slug)) {
          ctx.addIssue({
            code: "custom",
            message: "install_slug is duplicated",
            path: ["entries", index, "install_slug"],
          });
        }
        slugs.add(slug);
      }
    });
}

export const skillFeedSchema = catalogFeedSchema("skills", skillEntrySchema);
export const extensionFeedSchema = catalogFeedSchema("extensions", extensionEntrySchema);
export const mcpFeedSchema = catalogFeedSchema("mcp", mcpEntrySchema);

export const skillEntries: SkillEntry[] = skillFeedSchema.parse(skillsFeed).entries;
export const extensionEntries: ExtensionEntry[] = extensionFeedSchema.parse(extensionsFeed).entries;
export const mcpEntries: MCPEntry[] = mcpFeedSchema.parse(mcpFeed).entries;

export function parseMarketplaceCatalog(kind: "skills", feed: unknown): SkillEntry[];
export function parseMarketplaceCatalog(kind: "extensions", feed: unknown): ExtensionEntry[];
export function parseMarketplaceCatalog(kind: "mcp", feed: unknown): MCPEntry[];
export function parseMarketplaceCatalog(kind: MarketplaceKind, feed: unknown): MarketplaceEntry[] {
  switch (kind) {
    case "skills":
      return skillFeedSchema.parse(feed).entries;
    case "extensions":
      return extensionFeedSchema.parse(feed).entries;
    case "mcp":
      return mcpFeedSchema.parse(feed).entries;
  }
}

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

export function marketplaceSearchCommand(kind: MarketplaceKind, entry: MarketplaceEntry): string {
  const cliKind = kind === "skills" ? "skill" : kind === "extensions" ? "extension" : "mcp";
  return `${MARKETPLACE_SEARCH_COMMAND} ${entry.entry_id} --kind ${cliKind}`;
}

/** The CLI owns installation; this exact command is valid only after the daemon finds the entry. */
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
