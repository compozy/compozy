import { z } from "zod";

import type { ThemePreference } from "@/systems/theme";

import {
  globalShortcutRegistrationSchema,
  type GlobalShortcutRegistrationStatus,
} from "./window-manager-global-shortcut-schema";

export type { GlobalShortcutRegistrationStatus };

export interface GlobalShortcutBindingWire {
  command_id: string;
  chord: string;
}

export interface GlobalShortcutRegistrationWire {
  command_id: string;
  intended_chord: string;
  active_chord?: string;
  status: GlobalShortcutRegistrationStatus;
  reason?: string;
  settings_url?: string;
}

/** Parameters of `files.openInEditor`: an absolute or `~/` path and the session's inside roots. */
export interface OpenInEditorParams {
  path: string;
  line?: number;
  column?: number;
  roots: string[];
}

export type OpenInEditorErrorCode =
  | "target_unknown"
  | "target_unavailable"
  | "not_found"
  | "not_a_file"
  | "editor_invalid"
  | "editor_unavailable"
  | "editor_not_configured"
  | "editor_failed";

export type OpenInEditorResult =
  | { status: "opened" }
  | { status: "cancelled" }
  | { status: "error"; code: OpenInEditorErrorCode; message: string };

export interface DesktopShellEventMap {
  "shell:summon": { command_id: string };
}

export interface CompozyShellBridge {
  readonly platform: string;
  on<Event extends keyof DesktopShellEventMap>(
    event: Event,
    listener: (payload: DesktopShellEventMap[Event]) => void
  ): () => void;
  readonly globalShortcuts: {
    sync(bindings: GlobalShortcutBindingWire[]): Promise<GlobalShortcutRegistrationWire[]>;
    status(): Promise<GlobalShortcutRegistrationWire[]>;
  };
  /**
   * Reports the theme preference so native chrome and window backgrounds follow
   * it. Absent on desktop shells that predate theming.
   */
  readonly theme?: {
    set(preference: ThemePreference): Promise<void>;
  };
  /**
   * Native OS dialogs. `pickDirectory` resolves the chosen absolute folder, or
   * `null` on cancel. Absent on desktop shells that predate it.
   */
  readonly dialogs?: {
    pickDirectory(options?: { default_path?: string }): Promise<string | null>;
  };
  /**
   * Opens a local file in the operator's editor. The shell confirms targets outside `roots`
   * natively and reads the editor command itself. Absent on desktop shells that predate it.
   */
  readonly files?: {
    listOpenTargets?(): Promise<OpenTarget[]>;
    openWith?(params: OpenInEditorParams & { target: string }): Promise<OpenInEditorResult>;
    openInEditor(params: OpenInEditorParams): Promise<OpenInEditorResult>;
  };
}

declare global {
  interface Window {
    compozyShell?: CompozyShellBridge;
  }
}

export function parseGlobalShortcutRegistrations(value: unknown): GlobalShortcutRegistrationWire[] {
  return z.array(globalShortcutRegistrationSchema).parse(value);
}

export function desktopShellBridge(): CompozyShellBridge | null {
  if (typeof window === "undefined") return null;
  return window.compozyShell ?? null;
}

export function isDesktopShell(): boolean {
  return desktopShellBridge() !== null;
}

export interface OpenTarget {
  id: string;
  label: string;
  kind: "editor" | "reveal";
  supportsLine: boolean;
}
