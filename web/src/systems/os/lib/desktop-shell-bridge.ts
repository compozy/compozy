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
