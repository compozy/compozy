/**
 * Transport for the `window-manager` settings section.
 *
 * Bindings and aliases are mutated here and nowhere else: the daemon owns the
 * keymap, decides which command loses a contested chord, and echoes the whole
 * section back, so the client never computes a binding outcome of its own
 * (ADR-006, US-022.AC-2).
 */
import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";

import {
  parseSettingsWindowManagerSection,
  type WindowManagerSettingsSection,
} from "../lib/window-manager-settings-section";
import type { WindowManagerShortcutMap } from "../lib/window-manager-types";

/** Reads and writes address one scope; mixing them would split binding truth. */
export interface WindowManagerSettingsScopeInput {
  /** Empty or null reads global scope — the daemon then answers for core ids only. */
  workspaceId: string | null;
}

export interface WindowManagerBindingUpdate extends WindowManagerSettingsScopeInput {
  shortcuts?: WindowManagerShortcutMap;
  aliases?: Readonly<Record<string, string>>;
  /** Transfers a contested chord or alias away from its current owner. */
  overwrite?: boolean;
}

/** The daemon replaces the map wholesale, so the wire copy is always complete. */
function shortcutsBody(shortcuts: WindowManagerShortcutMap | undefined) {
  if (shortcuts === undefined) return undefined;
  return Object.fromEntries(
    Object.entries(shortcuts).map(([commandId, binding]) => [commandId, [...binding]])
  );
}

export type WindowManagerMutationCode = "shortcut_conflict" | "alias_conflict" | "invalid_alias";

/**
 * A refused binding mutation, carrying the daemon's own naming of the clash.
 *
 * `owner` is a command id: the settings surface resolves it to a title through
 * the section's command list rather than inventing a label for it.
 */
export class WindowManagerSettingsError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: WindowManagerMutationCode | null = null,
    public readonly owner: string | null = null,
    public readonly chord: string | null = null,
    public readonly alias: string | null = null
  ) {
    super(message);
    this.name = "WindowManagerSettingsError";
  }
}

function scopeQuery({ workspaceId }: WindowManagerSettingsScopeInput) {
  const normalized = workspaceId?.trim() ?? "";
  if (normalized === "") return { scope: "global" } as const;
  return { scope: "workspace", workspace_id: normalized } as const;
}

function mutationCode(value: unknown): WindowManagerMutationCode | null {
  return value === "shortcut_conflict" || value === "alias_conflict" || value === "invalid_alias"
    ? value
    : null;
}

function optionalText(value: unknown): string | null {
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function mutationError(response: Response, error: unknown, fallback: string) {
  const payload = error != null && typeof error === "object" ? error : {};
  const code = mutationCode(Reflect.get(payload, "error"));
  const message = optionalText(Reflect.get(payload, "message"));
  return new WindowManagerSettingsError(
    message ?? fallback,
    response.status,
    code,
    optionalText(Reflect.get(payload, "owner")),
    optionalText(Reflect.get(payload, "chord")),
    optionalText(Reflect.get(payload, "alias"))
  );
}

export async function fetchWindowManagerSettings(
  scope: WindowManagerSettingsScopeInput,
  signal?: AbortSignal
): Promise<WindowManagerSettingsSection> {
  const { data, error, response } = await apiClient.GET("/api/settings/window-manager", {
    params: { query: scopeQuery(scope) },
    signal,
  });
  const fallback = "Unable to load window-management settings.";
  if (apiRequestFailed(response, error)) {
    throw mutationError(response, error, fallback);
  }
  return parseSettingsWindowManagerSection(requireResponseData(data, response, fallback));
}

/**
 * Applies a whole desired map — the daemon replaces `shortcuts`/`aliases`
 * wholesale — and returns the section it produced, so callers never have to
 * predict the result of an overwrite.
 */
export async function updateWindowManagerBindings(
  update: WindowManagerBindingUpdate,
  signal?: AbortSignal
): Promise<WindowManagerSettingsSection> {
  const { data, error, response } = await apiClient.PATCH("/api/settings/window-manager", {
    body: {
      shortcuts: shortcutsBody(update.shortcuts),
      aliases: update.aliases === undefined ? undefined : { ...update.aliases },
      ...(update.overwrite === true ? { overwrite: true } : {}),
    },
    params: { query: scopeQuery(update) },
    signal,
  });
  const fallback = "Unable to save the keyboard shortcut.";
  if (apiRequestFailed(response, error)) {
    throw mutationError(response, error, fallback);
  }
  return parseSettingsWindowManagerSection(requireResponseData(data, response, fallback));
}
