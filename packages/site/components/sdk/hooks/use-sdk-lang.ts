"use client";

/**
 * The language the SDK reference is read in, shared by every SDK page and kept in `localStorage`
 * so it survives navigation and reloads. The server and the hydrating render read TypeScript;
 * React then re-renders with the stored choice, so a stored "Go" never mismatches the HTML.
 */
import { useSyncExternalStore } from "react";

import { isSdkLang, type SdkLang } from "@/lib/sdk-reference-model";

const STORAGE_KEY = "compozy.sdk.lang";
const DEFAULT_LANG: SdkLang = "ts";
const listeners = new Set<() => void>();
/** The choice made in this tab while storage refuses writes; unset whenever storage holds it. */
let chosen: SdkLang | null = null;

function readStored(): SdkLang {
  if (chosen) return chosen;
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    return isSdkLang(stored) ? stored : DEFAULT_LANG;
  } catch {
    return DEFAULT_LANG;
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  const onStorage = (event: StorageEvent) => {
    if (event.key !== STORAGE_KEY) return;
    // Another tab changed the language: follow it.
    chosen = null;
    listener();
  };
  window.addEventListener("storage", onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", onStorage);
  };
}

export function useSdkLang(): SdkLang {
  return useSyncExternalStore(subscribe, readStored, () => DEFAULT_LANG);
}

export function setSdkLang(lang: SdkLang): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, lang);
    // Storage holds the choice: read it from there so every tab agrees.
    chosen = null;
  } catch {
    // Storage can be unavailable (private mode); the choice then lasts for this tab only.
    chosen = lang;
  }
  for (const listener of listeners) listener();
}
