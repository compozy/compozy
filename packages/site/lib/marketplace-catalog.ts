import { z } from "zod";
import extensionsFeed from "../../../catalog/extensions.json";
import mcpFeed from "../../../catalog/mcp.json";
import skillsFeed from "../../../catalog/skills.json";
import {
  isExactDockerImageName,
  isExactMCPPackageName,
  isMCPLaunchArgument,
  isPublicMCPRemoteURL,
  isVerifiedDockerDigest,
  mcpRemoteURLQueryNames,
} from "./marketplace-catalog-validation";

/**
 * Build-time validation mirror of `internal/marketplace`. This validates the checked-in catalog
 * snapshot rendered by the site; a running daemon can instead use its configured, active source.
 */

const ENTRY_ID_PATTERN = /^[A-Za-z0-9._~-]+$/;
const ENV_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;
const SHA256_PATTERN = /^[a-f0-9]{64}$/i;
const SEMVER_PATTERN = /^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;
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
    format: nonBlankString
      .transform(value => value.toLowerCase())
      .pipe(z.enum(["compozy", "agent-plugin"]))
      .optional(),
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

const mcpLaunchArgs = z
  .array(z.string().refine(isMCPLaunchArgument, { message: "must be a non-empty argument" }))
  .optional();

const mcpLaunchSchema = z.discriminatedUnion("type", [
  z.strictObject({
    type: z.literal("npm"),
    package: z.string().refine(isExactMCPPackageName, {
      message: "must be an exact package name",
    }),
    version: nonBlankString.refine(value => SEMVER_PATTERN.test(value), {
      message: "must be an exact semantic version",
    }),
    args: mcpLaunchArgs,
  }),
  z.strictObject({
    type: z.literal("uvx"),
    package: z.string().refine(isExactMCPPackageName, {
      message: "must be an exact package name",
    }),
    version: nonBlankString.refine(value => SEMVER_PATTERN.test(value), {
      message: "must be an exact semantic version",
    }),
    args: mcpLaunchArgs,
  }),
  z.strictObject({
    type: z.literal("docker"),
    image: z.string().refine(isExactDockerImageName, {
      message: "must be an exact untagged image name",
    }),
    digest: nonBlankString.refine(isVerifiedDockerDigest, {
      message: "must be a verified sha256 digest",
    }),
    args: mcpLaunchArgs,
  }),
  z.strictObject({
    type: z.literal("remote"),
    url: nonBlankString.refine(isPublicMCPRemoteURL, {
      message: "must target a public HTTPS destination",
    }),
  }),
]);

const mcpAuthSchema = z
  .strictObject({
    method: z.literal("oauth"),
    registration: z.literal("auto"),
    scopes: z.array(nonBlankString).optional(),
  })
  .superRefine((auth, ctx) => {
    const seen = new Set<string>();
    for (const [index, scope] of (auth.scopes ?? []).entries()) {
      if (seen.has(scope)) {
        ctx.addIssue({
          code: "custom",
          message: "scopes must be unique",
          path: ["scopes", index],
        });
      }
      seen.add(scope);
    }
  });

const mcpInputSchema = z
  .strictObject({
    id: entryID,
    prompt: nonBlankString,
    type: z.enum(["string", "identifier", "boolean", "secret"]),
    required: z.boolean(),
    binding: z.strictObject({
      type: z.enum(["env", "url_query"]),
      name: nonBlankString,
    }),
    default: z.union([z.string(), z.boolean()]).optional(),
  })
  .superRefine((input, ctx) => {
    if (input.type === "secret" && input.default !== undefined) {
      ctx.addIssue({
        code: "custom",
        message: "secret inputs must not set default",
        path: ["default"],
      });
    }
    if (
      input.type === "boolean" &&
      input.default !== undefined &&
      typeof input.default !== "boolean"
    ) {
      ctx.addIssue({
        code: "custom",
        message: "boolean defaults must be boolean",
        path: ["default"],
      });
    }
    if (
      (input.type === "string" || input.type === "identifier") &&
      input.default !== undefined &&
      typeof input.default !== "string"
    ) {
      ctx.addIssue({
        code: "custom",
        message: `${input.type} defaults must be strings`,
        path: ["default"],
      });
    }
    if (
      input.type === "identifier" &&
      typeof input.default === "string" &&
      input.default.trim() === ""
    ) {
      ctx.addIssue({
        code: "custom",
        message: "identifier defaults must be non-empty strings",
        path: ["default"],
      });
    }
    if (
      input.binding.type === "env" &&
      !isMCPEnvironmentName(input.binding.name, input.type === "secret")
    ) {
      ctx.addIssue({
        code: "custom",
        message: "env binding must be permitted",
        path: ["binding", "name"],
      });
    }
    if (input.binding.type === "url_query" && !ENTRY_ID_PATTERN.test(input.binding.name)) {
      ctx.addIssue({
        code: "custom",
        message: "url_query binding must be URL-safe",
        path: ["binding", "name"],
      });
    }
  });

export const mcpEntrySchema = z
  .strictObject({
    ...entryCommon,
    launch: mcpLaunchSchema,
    auth: mcpAuthSchema.optional(),
    inputs: z.array(mcpInputSchema).optional(),
    default_scope: z.enum(["workspace", "global"]),
  })
  .superRefine((entry, ctx) => {
    if (entry.auth && entry.launch.type !== "remote") {
      ctx.addIssue({
        code: "custom",
        message: "auth is only allowed for remote launch",
        path: ["auth"],
      });
    }
    const seen = new Set<string>();
    const seenBindings = new Set<string>();
    const launchQueryNames =
      entry.launch.type === "remote" ? mcpRemoteURLQueryNames(entry.launch.url) : new Set<string>();
    for (const [index, input] of (entry.inputs ?? []).entries()) {
      if (seen.has(input.id)) {
        ctx.addIssue({
          code: "custom",
          message: "input ids must be unique",
          path: ["inputs", index, "id"],
        });
      }
      seen.add(input.id);
      const { name: bindingName, type: bindingType } = input.binding;
      const binding = `${bindingType}\0${bindingName}`;
      if (seenBindings.has(binding)) {
        ctx.addIssue({
          code: "custom",
          message: `input binding ${bindingType}/${bindingName} is duplicated`,
          path: ["inputs", index, "binding"],
        });
      }
      seenBindings.add(binding);
      if (bindingType === "env" && entry.launch.type === "remote") {
        ctx.addIssue({
          code: "custom",
          message: "env inputs are only allowed for local launch (npm, uvx, docker)",
          path: ["inputs", index, "binding"],
        });
      }
      if (bindingType === "url_query" && entry.launch.type !== "remote") {
        ctx.addIssue({
          code: "custom",
          message: "url_query inputs are only allowed for remote launch",
          path: ["inputs", index, "binding"],
        });
      }
      if (bindingType === "url_query" && input.type === "secret") {
        ctx.addIssue({
          code: "custom",
          message: "secret inputs cannot bind url_query",
          path: ["inputs", index, "binding"],
        });
      }
      if (bindingType === "url_query" && launchQueryNames.has(bindingName)) {
        ctx.addIssue({
          code: "custom",
          message: `input binding url_query/${bindingName} conflicts with launch URL`,
          path: ["inputs", index, "binding"],
        });
      }
    }
  });

export type SkillEntry = z.infer<typeof skillEntrySchema>;
export type ExtensionEntry = z.infer<typeof extensionEntrySchema>;
export type MCPEntry = z.infer<typeof mcpEntrySchema>;

export type MarketplaceKind = "skills" | "extensions" | "mcp";
export type MarketplaceEntry = SkillEntry | ExtensionEntry | MCPEntry;

interface MarketplaceEntryByKind {
  skills: SkillEntry;
  extensions: ExtensionEntry;
  mcp: MCPEntry;
}

export const MARKETPLACE_KINDS: readonly MarketplaceKind[] = ["skills", "extensions", "mcp"];
export const MARKETPLACE_FEED_FILENAMES = ["skills.json", "extensions.json", "mcp.json"] as const;
export const MARKETPLACE_SEARCH_COMMAND = "compozy marketplace search";

type FeedEntry = { entry_id: string; install_slug?: string };

function catalogFeedSchema<Entry extends z.ZodType>(kind: MarketplaceKind, entry: Entry) {
  return z
    .strictObject({
      manifest_version: z.literal(2),
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

const installCommandByKind: {
  [Kind in MarketplaceKind]: (entry: MarketplaceEntryByKind[Kind]) => string;
} = {
  skills: entry => `compozy skill install ${entry.install_slug}`,
  extensions: entry => `compozy extension install ${entry.install_slug}`,
  mcp: entry => `compozy mcp install ${entry.entry_id}`,
};

/** The CLI owns installation; this exact command is valid only after the daemon finds the entry. */
export function installCommand<Kind extends MarketplaceKind>(
  kind: Kind,
  entry: MarketplaceEntryByKind[Kind]
): string {
  return installCommandByKind[kind](entry);
}
