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

function readStored(): SdkLang {
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
    if (event.key === STORAGE_KEY) listener();
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
  } catch {
    // Storage can be unavailable (private mode); the choice then lasts for this render only.
  }
  for (const listener of listeners) listener();
}
