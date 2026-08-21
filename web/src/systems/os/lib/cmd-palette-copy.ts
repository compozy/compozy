import type { CmdPaletteAction } from "./cmd-palette-types";

/** Matches `cmdpalette.MaxViewTextBytes`. */
export const PALETTE_COPY_MAX_BYTES = 4 * 1024;
export const COPY_ACTION_CONTENT_KEY = "content";
export const COPY_REQUIRES_TARGET_REASON = 'action "copy" requires its target';
export const COPY_CONTENT_TOO_LARGE_REASON = `action "copy" content exceeds ${PALETTE_COPY_MAX_BYTES} bytes`;
export const CLIPBOARD_UNAVAILABLE_REASON = "Clipboard access is unavailable.";

const EXTRA_COPY_FIELD_TARGETS = [
  ["op", "client_op"],
  ["tool", "tool"],
  ["view", "view"],
  ["app", "navigate"],
  ["url", "url"],
] as const;

export function copyCannotCarryReason(target: string): string {
  return `action "copy" cannot carry "${target}" target`;
}

export type PaletteCopyResolution =
  | { readonly ok: true; readonly content: string }
  | { readonly ok: false; readonly reason: string };

/**
 * Host-side copy policy. The daemon already rejects these shapes at admit time;
 * the seam repeats the same gates so a synthesized view action cannot fall
 * through to invoke or write the clipboard.
 */
export function resolvePaletteCopyContent(
  action: CmdPaletteAction,
  args: Readonly<Record<string, unknown>>
): PaletteCopyResolution {
  for (const [field, target] of EXTRA_COPY_FIELD_TARGETS) {
    const value = Reflect.get(action, field);
    if (typeof value === "string" && value.trim() !== "") {
      return { ok: false, reason: copyCannotCarryReason(target) };
    }
  }
  const extras = Object.keys(args)
    .filter(key => key !== COPY_ACTION_CONTENT_KEY)
    .sort();
  const extra = extras[0];
  if (extra !== undefined) {
    return { ok: false, reason: copyCannotCarryReason(extra) };
  }
  const raw = args[COPY_ACTION_CONTENT_KEY];
  if (typeof raw !== "string") {
    return { ok: false, reason: COPY_REQUIRES_TARGET_REASON };
  }
  const content = raw.trim();
  if (content === "") {
    return { ok: false, reason: COPY_REQUIRES_TARGET_REASON };
  }
  if (new TextEncoder().encode(content).byteLength > PALETTE_COPY_MAX_BYTES) {
    return { ok: false, reason: COPY_CONTENT_TOO_LARGE_REASON };
  }
  return { ok: true, content };
}

export async function writePaletteClipboard(content: string): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (clipboard === undefined || typeof clipboard.writeText !== "function") {
    throw new Error(CLIPBOARD_UNAVAILABLE_REASON);
  }
  await clipboard.writeText(content);
}
