import { readFile } from "node:fs/promises";

import { writeFileAtomic } from "../files/atomic-write";
import { clampZoomLevel } from "./zoom-policy";
import { parseWindowState, type PersistedWindowState } from "./window-state-policy";

export interface StoredWindowState extends PersistedWindowState {
  readonly zoom_level: number;
}

export async function readWindowState(path: string): Promise<StoredWindowState | null> {
  try {
    const value: unknown = JSON.parse(await readFile(path, "utf8"));
    const state = parseWindowState(value);
    if (!state) return null;
    const zoom =
      value && typeof value === "object" ? (value as Record<string, unknown>).zoom_level : 0;
    return { ...state, zoom_level: typeof zoom === "number" ? clampZoomLevel(zoom) : 0 };
  } catch {
    return null;
  }
}

export async function writeWindowState(path: string, state: StoredWindowState): Promise<void> {
  await writeFileAtomic(path, `${JSON.stringify(state, null, 2)}\n`, 0o600);
}
