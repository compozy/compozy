import { z } from "zod";
import {
  catalogFeedSchema,
  extensionEntrySchema,
  mcpInputSchema,
  rfc3339,
} from "./marketplace-catalog-schema";

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

const extensionInputsSchema = z.array(mcpInputSchema).superRefine((inputs, ctx) => {
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

export const extensionV3EntrySchema = extensionEntrySchema.safeExtend({
  icon: z
    .string()
    .refine(isCatalogIcon, {
      message: "icon must be a PNG, SVG or WebP HTTPS/data URL within 64 KiB",
    })
    .optional(),
  inputs: extensionInputsSchema.optional(),
});
export const extensionV3FeedSchema = catalogFeedSchema("extensions", extensionV3EntrySchema, 3);

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
