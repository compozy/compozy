/**
 * The `window-manager` settings section as the client reads it.
 *
 * The daemon serves the whole bindable registry here — every command with its
 * title, section and source, the effective keymap it resolved, and the aliases
 * in force — so no surface has to assemble a command list of its own (ADR-006).
 * Read at workspace scope the list spans core plus `ext.*`; read globally the
 * daemon can only answer for core ids, and says so by titling them with their id.
 */
import { z } from "zod";

import {
  shortcutsSchema,
  toWindowManagerConfig,
  windowManagerWireConfigSchema,
} from "./window-manager-config-schema";
import type { ShortcutBinding } from "./window-manager-shortcuts";
import type { WindowManagerConfig } from "./window-manager-types";

export type WindowManagerSettingsScope = "global" | "workspace";

/** One bindable command, exactly as the registry names it. */
export interface WindowManagerShortcutCommand {
  id: string;
  title: string;
  section: string;
  /** `core` or `ext.<name>`, verbatim from the registry. */
  source: string;
}

/**
 * An extension's declared default binding. `dormant` means the daemon withheld
 * it because `conflictWith` already owns the chord (US-029.AC-2).
 */
export interface WindowManagerExtensionDefault {
  commandId: string;
  binding: ShortcutBinding;
  dormant: boolean;
  conflictWith: string | null;
}

/** A stored override the daemon tolerated but could not resolve (US-022.EC-3). */
export interface WindowManagerShortcutDiagnostic {
  commandId: string;
  message: string;
}

export type WindowManagerAliasMap = Readonly<Record<string, string>>;

export interface WindowManagerSettingsSection {
  scope: WindowManagerSettingsScope;
  availableScopes: readonly WindowManagerSettingsScope[];
  workspaceId: string | null;
  config: WindowManagerConfig;
  commands: readonly WindowManagerShortcutCommand[];
  aliases: WindowManagerAliasMap;
  extensionDefaults: readonly WindowManagerExtensionDefault[];
  diagnostics: readonly WindowManagerShortcutDiagnostic[];
}

const scopeSchema = z.enum(["global", "workspace"]);

const commandSchema = z.strictObject({
  id: z.string(),
  title: z.string(),
  section: z.string(),
  source: z.string(),
});

const extensionDefaultSchema = z.strictObject({
  command: z.string(),
  binding: z.union([z.string(), z.array(z.string())]),
  dormant: z.boolean(),
  conflict_with: z.string().optional(),
});

const diagnosticSchema = z.strictObject({
  command_id: z.string(),
  message: z.string(),
});

const settingsWindowManagerSectionSchema = z.strictObject({
  // Typed as the daemon's open section enum so the generated payload stays
  // assignable here, then pinned at runtime: parsing another section's envelope
  // as this one would be a routing bug, not a tolerable variation.
  section: z.string().refine(value => value === "window-manager", {
    message: "not the window-manager settings section",
  }),
  scope: scopeSchema,
  available_scopes: z.array(scopeSchema),
  workspace_id: z.string().optional(),
  config: windowManagerWireConfigSchema,
  defaults: shortcutsSchema,
  effective_shortcuts: shortcutsSchema,
  aliases: z.record(z.string(), z.string()),
  commands: z.array(commandSchema),
  extension_defaults: z.array(extensionDefaultSchema),
  diagnostics: z.array(diagnosticSchema).optional(),
});

/**
 * The generated payload type. Parsers take it rather than `unknown` so a
 * contract change fails typecheck at the call site instead of at runtime.
 */
export type WindowManagerSettingsWire = z.input<typeof settingsWindowManagerSectionSchema>;

function toBinding(binding: string | string[]): ShortcutBinding {
  if (typeof binding === "string") return binding.trim() === "" ? [] : [binding];
  return binding;
}

export function parseSettingsWindowManagerSection(
  payload: WindowManagerSettingsWire
): WindowManagerSettingsSection {
  const response = settingsWindowManagerSectionSchema.parse(payload);
  return {
    scope: response.scope,
    availableScopes: response.available_scopes,
    workspaceId: response.workspace_id ?? null,
    config: toWindowManagerConfig(response.config, response.defaults, response.effective_shortcuts),
    commands: response.commands,
    aliases: response.aliases,
    extensionDefaults: response.extension_defaults.map(entry => ({
      commandId: entry.command,
      binding: toBinding(entry.binding),
      dormant: entry.dormant,
      conflictWith: entry.conflict_with ?? null,
    })),
    diagnostics: (response.diagnostics ?? []).map(entry => ({
      commandId: entry.command_id,
      message: entry.message,
    })),
  };
}

export function parseSettingsWindowManagerConfig(
  payload: WindowManagerSettingsWire
): WindowManagerConfig {
  return parseSettingsWindowManagerSection(payload).config;
}
