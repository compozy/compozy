import "@testing-library/jest-dom";
import { beforeEach } from "vitest";

function createMemoryStorage(): Storage {
  const store = new Map<string, string>();

  return {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key: string) {
      return store.get(key) ?? null;
    },
    key(index: number) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key: string) {
      store.delete(key);
    },
    setItem(key: string, value: string) {
      store.set(key, value);
    },
  };
}

function storageIsUsable(storage: Storage | undefined): storage is Storage {
  if (!storage) return false;
  try {
    const probeKey = "__agh_test_storage_probe__";
    storage.setItem(probeKey, "1");
    storage.removeItem(probeKey);
    return true;
  } catch {
    return false;
  }
}

function ensureWindowStorage(name: "localStorage" | "sessionStorage") {
  let storage: Storage | undefined;
  try {
    storage = window[name];
  } catch {
    storage = undefined;
  }
  if (storageIsUsable(storage)) return;

  Object.defineProperty(window, name, {
    configurable: true,
    value: createMemoryStorage(),
    writable: true,
  });
}

if (typeof window !== "undefined") {
  ensureWindowStorage("localStorage");
  ensureWindowStorage("sessionStorage");

  beforeEach(() => {
    window.localStorage.clear();
    window.sessionStorage.clear();
  });

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });

  class ResizeObserverMock {
    observe() {}
    unobserve() {}
    disconnect() {}
  }

  window.ResizeObserver = ResizeObserverMock;
  window.scrollTo = () => {};

  if (typeof Element !== "undefined" && !Element.prototype.getAnimations) {
    Element.prototype.getAnimations = function getAnimations() {
      return [];
    };
  }
  if (typeof Element !== "undefined" && !Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = function scrollIntoView() {};
  }
  if (typeof Element !== "undefined" && !Element.prototype.scrollTo) {
    Element.prototype.scrollTo = function scrollTo() {};
  }
}
