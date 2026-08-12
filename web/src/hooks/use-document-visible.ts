import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();

function notifyVisibilityChange(): void {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  if (typeof document === "undefined") return () => undefined;
  listeners.add(listener);
  if (listeners.size === 1) {
    document.addEventListener("visibilitychange", notifyVisibilityChange);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      document.removeEventListener("visibilitychange", notifyVisibilityChange);
    }
  };
}

function getSnapshot(): boolean {
  return typeof document === "undefined" || document.visibilityState === "visible";
}

export function useDocumentVisible(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => true);
}
