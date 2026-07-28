import { useEffect, useState } from "react";

interface KeyedLocalStoragePreferenceOptions<T> {
  storageKey: string;
  itemKey: string;
  defaultValue: T;
  decode: (value: unknown) => T | null;
}

function readStore<T>(storageKey: string, decode: (value: unknown) => T | null): Record<string, T> {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    const entries = Object.entries(parsed).flatMap(([key, value]) => {
      const decoded = decode(value);
      return decoded === null ? [] : [[key, decoded] as const];
    });
    return Object.fromEntries(entries);
  } catch {
    return {};
  }
}

function writeStore<T>(storageKey: string, store: Record<string, T>): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(store));
  } catch {
    // Preferences are best-effort when storage is unavailable or full.
  }
}

/** One defensive lifecycle for keyed, cross-tab localStorage preferences. */
export function useKeyedLocalStoragePreference<T>({
  storageKey,
  itemKey,
  defaultValue,
  decode,
}: KeyedLocalStoragePreferenceOptions<T>): [T, (next: T) => void] {
  const [stored, setStored] = useState<{ key: string; value: T }>(() => ({
    key: itemKey,
    value: itemKey ? (readStore(storageKey, decode)[itemKey] ?? defaultValue) : defaultValue,
  }));
  const value =
    stored.key === itemKey
      ? stored.value
      : itemKey
        ? (readStore(storageKey, decode)[itemKey] ?? defaultValue)
        : defaultValue;

  useEffect(() => {
    if (typeof window === "undefined") return undefined;
    const handleStorage = (event: StorageEvent) => {
      if (event.key !== storageKey) return;
      setStored({
        key: itemKey,
        value: itemKey ? (readStore(storageKey, decode)[itemKey] ?? defaultValue) : defaultValue,
      });
    };
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, [decode, defaultValue, itemKey, storageKey]);

  const persist = (next: T) => {
    setStored({ key: itemKey, value: next });
    if (!itemKey) return;
    const store = readStore(storageKey, decode);
    store[itemKey] = next;
    writeStore(storageKey, store);
  };

  return [value, persist];
}
