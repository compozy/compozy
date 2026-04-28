import "@testing-library/jest-dom/vitest";

import { afterEach, vi } from "vitest";

import { setWorkspaceEventStreamFactoryOverrideForTests } from "@/systems/app-shell/lib/workspace-events";

setWorkspaceEventStreamFactoryOverrideForTests(() => ({ close: () => {} }));

if (typeof window !== "undefined") {
  window.scrollTo = vi.fn();
  if (!HTMLElement.prototype.scrollTo) {
    HTMLElement.prototype.scrollTo = vi.fn();
  }
  if (typeof globalThis.ResizeObserver === "undefined") {
    class TestResizeObserver implements ResizeObserver {
      observe(): void {
        return undefined;
      }
      unobserve(): void {
        return undefined;
      }
      disconnect(): void {
        return undefined;
      }
    }
    Object.defineProperty(window, "ResizeObserver", {
      configurable: true,
      value: TestResizeObserver,
    });
    Object.defineProperty(globalThis, "ResizeObserver", {
      configurable: true,
      value: TestResizeObserver,
    });
  }
}

afterEach(() => {
  if (typeof document !== "undefined") {
    document.cookie.split(";").forEach(entry => {
      const name = entry.split("=")[0]?.trim();
      if (name) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
      }
    });
  }
  if (typeof window !== "undefined") {
    try {
      window.sessionStorage.clear();
      window.localStorage.clear();
    } catch {
      // ignore — jsdom may disable storage in some environments
    }
  }
});
