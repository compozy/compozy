import { z } from "zod";

const ENTRY_ID_PATTERN = /^[A-Za-z0-9._~-]+$/;
const ENV_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;
const SHA256_PATTERN = /^[a-f0-9]{64}$/i;
const RFC3339_PATTERN =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(Z|[+-]\d{2}:\d{2})$/;
const MAX_CATALOG_ENTRIES = 50_000;

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

function isInputEnvironmentName(value: string, secret: boolean): boolean {
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

export const rfc3339 = trimmedString.refine(isRFC3339, { message: "must be an RFC3339 timestamp" });
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

const baseExtensionEntrySchema = z
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

const inputSchema = z
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
      !isInputEnvironmentName(input.binding.name, input.type === "secret")
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

const bytes = new TextEncoder();
const identifier = /^[A-Za-z0-9._~-]+$/;

/** Mirrors the published icon-reference contract; rendering still owns its loading fallback. */
function isCatalogIcon(value: string): boolean {
  if (!value) return true;
  if (bytes.encode(value).length > 64 * 1024) return false;
  try {
    if (value.startsWith("data:")) {
      const match = /^data:(image\/(?:png|svg\+xml|webp))(;base64)?,(.+)$/.exec(value);
      if (!match) return false;
      if (match[2]) {
        const data = match[3].replace(/[\r\n]/g, "");
        if (!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(data))
          return false;
        atob(data);
      } else {
        decodeURIComponent(match[3]);
      }
      return true;
    }
    const url = new URL(value);
    return (
      url.protocol === "https:" &&
      !!url.hostname &&
      !url.username &&
      !url.password &&
      !url.hash &&
      /\.(?:png|svg|webp)$/i.test(url.pathname)
    );
  } catch {
    return false;
  }
}

const extensionInputsSchema = z.array(inputSchema).superRefine((inputs, ctx) => {
  const ids = new Set<string>();
  const bindings = new Set<string>();
  for (const [index, input] of inputs.entries()) {
    const binding = `${input.binding.type}\0${input.binding.name}`;
    if (ids.has(input.id))
      ctx.addIssue({ code: "custom", message: "input ids must be unique", path: [index, "id"] });
    if (bindings.has(binding))
      ctx.addIssue({
        code: "custom",
        message: "input bindings must be unique",
        path: [index, "binding"],
      });
    ids.add(input.id);
    bindings.add(binding);
    if (input.type === "secret" && input.binding.type === "url_query") {
      ctx.addIssue({
        code: "custom",
        message: "secret inputs cannot bind url_query",
        path: [index, "binding"],
      });
    }
    if (typeof input.default === "string") {
      if (input.default.includes("\0") || bytes.encode(input.default).length > 8192) {
        ctx.addIssue({
          code: "custom",
          message: "default must be NUL-free and at most 8 KiB",
          path: [index, "default"],
        });
      }
      if (input.type === "identifier" && !identifier.test(input.default.trim())) {
        ctx.addIssue({
          code: "custom",
          message: "identifier default must be URL-safe",
          path: [index, "default"],
        });
      }
    }
  }
});

export const extensionEntrySchema = baseExtensionEntrySchema.safeExtend({
  icon: z
    .string()
    .refine(isCatalogIcon, {
      message: "icon must be a PNG, SVG or WebP HTTPS/data URL within 64 KiB",
    })
    .optional(),
  inputs: extensionInputsSchema.optional(),
});
export const extensionFeedSchema = z
  .strictObject({
    manifest_version: z.literal(3),
    generated_at: rfc3339,
    entries: z.array(extensionEntrySchema).min(1).max(MAX_CATALOG_ENTRIES),
  })
  .superRefine((feed, ctx) => {
    const ids = new Set<string>();
    const slugs = new Set<string>();
    for (const [index, entry] of feed.entries.entries()) {
      if (ids.has(entry.entry_id))
        ctx.addIssue({
          code: "custom",
          message: "entry_id is duplicated",
          path: ["entries", index, "entry_id"],
        });
      if (slugs.has(entry.install_slug))
        ctx.addIssue({
          code: "custom",
          message: "install_slug is duplicated",
          path: ["entries", index, "install_slug"],
        });
      ids.add(entry.entry_id);
      slugs.add(entry.install_slug);
    }
  });

export const marketplacePresetsSchema = z
  .strictObject({
    manifest_version: z.literal(3),
    generated_at: rfc3339,
    entries: z.array(
      z.strictObject({
        name: z
          .string()
          .regex(/^[a-z0-9._-]{1,64}$/)
          .refine(name => name !== "compozy" && name !== "compozy-catalog", {
            message: "preset name is reserved",
          }),
        source: z.string().trim().min(1),
        description: z.string().trim().min(1),
        default: z.enum(["on", "off"]),
      })
    ),
  })
  .superRefine((feed, ctx) => {
    const names = new Set<string>();
    for (const [index, entry] of feed.entries.entries()) {
      if (names.has(entry.name))
        ctx.addIssue({
          code: "custom",
          message: "preset names must be unique",
          path: ["entries", index, "name"],
        });
      names.add(entry.name);
    }
  });
