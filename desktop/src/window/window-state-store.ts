import type { SpellcheckSettings } from "./context-menu";

import { readFile } from "node:fs/promises";

import { writeFileAtomic } from "../files/atomic-write";
import { clampZoomLevel } from "./zoom-policy";
import { parseWindowState, type PersistedWindowState } from "./window-state-policy";

export interface StoredWindowState extends PersistedWindowState {
  readonly zoom_level: number;
  readonly spellcheck?: SpellcheckSettings;
}

export async function readWindowState(path: string): Promise<StoredWindowState | null> {
  try {
    const value: unknown = JSON.parse(await readFile(path, "utf8"));
    const state = parseWindowState(value);
    if (!state) return null;
    const zoom =
      value && typeof value === "object" ? (value as Record<string, unknown>).zoom_level : 0;
    const spelling =
      value && typeof value === "object" ? Reflect.get(value, "spellcheck") : undefined;
    const spellcheck =
      spelling &&
      typeof spelling === "object" &&
      typeof spelling.enabled === "boolean" &&
      Array.isArray(spelling.languages) &&
      spelling.languages.every((language: unknown) => typeof language === "string")
        ? { enabled: spelling.enabled, languages: [...new Set<string>(spelling.languages)] }
        : undefined;
    return {
      ...state,
      zoom_level: typeof zoom === "number" ? clampZoomLevel(zoom) : 0,
      ...(spellcheck ? { spellcheck } : {}),
    };
  } catch {
    return null;
  }
}

export async function writeWindowState(path: string, state: StoredWindowState): Promise<void> {
  await writeFileAtomic(path, `${JSON.stringify(state, null, 2)}\n`, 0o600);
}
