/**
 * Bridges the curated catalog selection and the runtime `event` string.
 *
 * `draft.event` is the single source of truth (a free-form runtime string such
 * as `session.stopped`, `hook.transform.completed`, `ext.release.deploy-started`,
 * or `webhook`). The catalog UI parses that string into a structured selection
 * and recomposes it whenever the hook/extension sub-config changes — no extra
 * component state, and edit-mode round-trips cleanly.
 */

import type { EventFamily } from "./trigger-catalog";
import { getEventDef } from "./trigger-catalog";

export interface EventSelection {
  /** Catalog id of the matched event, or "" when the event is not in the catalog. */
  catalogId: string;
  family: EventFamily;
  hookName: string;
  extExt: string;
  extEvent: string;
}

const HOOK_PREFIX = "hook.";
const HOOK_SUFFIX = ".completed";
const EXT_PREFIX = "ext.";

export function parseEventSelection(event: string): EventSelection {
  const trimmed = event.trim();

  if (trimmed === "webhook") {
    return { catalogId: "webhook", family: "webhook", hookName: "", extExt: "", extEvent: "" };
  }

  if (trimmed.startsWith(HOOK_PREFIX) && trimmed.endsWith(HOOK_SUFFIX)) {
    const hookName = trimmed.slice(HOOK_PREFIX.length, trimmed.length - HOOK_SUFFIX.length);
    return { catalogId: "hook.completed", family: "hook", hookName, extExt: "", extEvent: "" };
  }

  if (trimmed.startsWith(EXT_PREFIX)) {
    const rest = trimmed.slice(EXT_PREFIX.length);
    const separator = rest.indexOf(".");
    const extExt = separator === -1 ? rest : rest.slice(0, separator);
    const extEvent = separator === -1 ? "" : rest.slice(separator + 1);
    return { catalogId: "ext", family: "ext", hookName: "", extExt, extEvent };
  }

  const def = getEventDef(trimmed);
  if (def) {
    return { catalogId: def.id, family: def.family, hookName: "", extExt: "", extEvent: "" };
  }

  // Unknown / custom runtime event: keep it usable without a catalog match.
  return { catalogId: "", family: "fixed", hookName: "", extExt: "", extEvent: "" };
}

/** Builds the literal runtime `event` string the daemon stores. */
export function composeEventId(selection: {
  family: EventFamily;
  hookName?: string;
  extExt?: string;
  extEvent?: string;
  catalogId?: string;
}): string {
  switch (selection.family) {
    case "webhook":
      return "webhook";
    case "hook":
      return `${HOOK_PREFIX}${selection.hookName ?? ""}${HOOK_SUFFIX}`;
    case "ext":
      return `${EXT_PREFIX}${selection.extExt ?? ""}.${selection.extEvent ?? ""}`;
    default:
      return selection.catalogId ?? "";
  }
}

/**
 * Display form of the event id with `<name>` / `<ext>` / `<event>` placeholders
 * for the parts the operator has not filled in yet (matches the summary and
 * sample-event preview).
 */
export function formatEventKind(selection: EventSelection, fallback = ""): string {
  switch (selection.family) {
    case "webhook":
      return "webhook";
    case "hook":
      return `${HOOK_PREFIX}${selection.hookName || "<name>"}${HOOK_SUFFIX}`;
    case "ext":
      return `${EXT_PREFIX}${selection.extExt || "<ext>"}.${selection.extEvent || "<event>"}`;
    default:
      return selection.catalogId || fallback;
  }
}

/** Concrete event id used inside the sample JSON (no placeholders). */
export function sampleEventKind(selection: EventSelection, sampleKind: string): string {
  switch (selection.family) {
    case "hook":
      return selection.hookName ? `${HOOK_PREFIX}${selection.hookName}${HOOK_SUFFIX}` : sampleKind;
    case "ext":
      return selection.extExt || selection.extEvent
        ? `${EXT_PREFIX}${selection.extExt || "ext"}.${selection.extEvent || "event"}`
        : sampleKind;
    default:
      return sampleKind;
  }
}
