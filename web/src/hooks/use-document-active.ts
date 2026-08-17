import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();

function notifyActivityChange(): void {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  if (typeof document === "undefined" || typeof window === "undefined") return () => undefined;
  listeners.add(listener);
  if (listeners.size === 1) {
    document.addEventListener("visibilitychange", notifyActivityChange);
    window.addEventListener("focus", notifyActivityChange);
    window.addEventListener("blur", notifyActivityChange);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      document.removeEventListener("visibilitychange", notifyActivityChange);
      window.removeEventListener("focus", notifyActivityChange);
      window.removeEventListener("blur", notifyActivityChange);
    }
  };
}

function getSnapshot(): boolean {
  return (
    typeof document === "undefined" ||
    (document.visibilityState === "visible" && document.hasFocus())
  );
}

export function useDocumentActive(): boolean {
  return useSyncExternalStore(subscribe, getSnapshot, () => true);
}
