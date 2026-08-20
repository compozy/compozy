import {
  GLOBAL_SHORTCUT_STATUS_VALUES,
  type GlobalShortcutBinding,
  type GlobalShortcutRegistration,
} from "../shortcuts/global-shortcut-types";

export const PRODUCT_METHOD_VALUES = ["global_shortcuts.sync", "global_shortcuts.status"] as const;
export type ProductMethod = (typeof PRODUCT_METHOD_VALUES)[number];
export const PRODUCT_METHODS = new Set<string>(PRODUCT_METHOD_VALUES);

export const PRODUCT_EVENT_VALUES = ["shell:summon"] as const;
export type ProductEvent = (typeof PRODUCT_EVENT_VALUES)[number];
export const PRODUCT_EVENTS = new Set<string>(PRODUCT_EVENT_VALUES);

const STATUSES = new Set<string>(GLOBAL_SHORTCUT_STATUS_VALUES);
const REGISTRATION_KEYS = new Set([
  "command_id",
  "intended_chord",
  "active_chord",
  "status",
  "reason",
  "settings_url",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validBinding(value: unknown): value is GlobalShortcutBinding {
  return (
    isRecord(value) &&
    Object.keys(value).length === 2 &&
    typeof value.command_id === "string" &&
    value.command_id.trim() !== "" &&
    typeof value.chord === "string" &&
    value.chord.trim() !== ""
  );
}

function validRegistration(value: unknown): value is GlobalShortcutRegistration {
  if (!isRecord(value)) return false;
  if (Object.keys(value).some(key => !REGISTRATION_KEYS.has(key))) return false;
  return (
    typeof value.command_id === "string" &&
    value.command_id.trim() !== "" &&
    typeof value.intended_chord === "string" &&
    value.intended_chord.trim() !== "" &&
    typeof value.status === "string" &&
    STATUSES.has(value.status) &&
    (value.active_chord === undefined || typeof value.active_chord === "string") &&
    (value.reason === undefined || typeof value.reason === "string") &&
    (value.settings_url === undefined || typeof value.settings_url === "string")
  );
}

export function validProductParams(method: ProductMethod, value: unknown): boolean {
  if (!isRecord(value)) return false;
  if (method === "global_shortcuts.status") return Object.keys(value).length === 0;
  return (
    Object.keys(value).length === 1 &&
    Array.isArray(value.bindings) &&
    value.bindings.every(validBinding)
  );
}

export function validProductResponse(
  _method: ProductMethod,
  value: unknown
): value is GlobalShortcutRegistration[] {
  return Array.isArray(value) && value.every(validRegistration);
}

export function validProductEventPayload(event: ProductEvent, value: unknown): boolean {
  if (event !== "shell:summon" || !isRecord(value)) return false;
  return (
    Object.keys(value).length === 1 &&
    typeof value.command_id === "string" &&
    value.command_id.trim() !== ""
  );
}

export type { GlobalShortcutBinding, GlobalShortcutRegistration };
