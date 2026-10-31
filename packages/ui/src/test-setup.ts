import * as matchers from "@testing-library/jest-dom/matchers";
import type { TestingLibraryMatchers } from "@testing-library/jest-dom/matchers";
import { expect } from "vitest";

// jest-dom 7 declares the pre-Vitest-5 assertion interface.
declare module "vitest" {
  interface Matchers<
    R extends void | Promise<void> = void | Promise<void>,
    T = unknown,
  > extends TestingLibraryMatchers<T, R> {}
}

expect.extend(matchers);

if (typeof window !== "undefined") {
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

  if (typeof Element !== "undefined" && !Element.prototype.getAnimations) {
    Element.prototype.getAnimations = function getAnimations() {
      return [];
    };
  }

  if (typeof Element !== "undefined" && !Element.prototype.scrollIntoView) {
    Element.prototype.scrollIntoView = function scrollIntoView() {};
  }

  // ProseMirror (the Markdown editor) measures the selection on every edit;
  // jsdom has no layout, so the measurements read as empty boxes.
  const emptyRects = () => Object.assign([], { item: () => null }) as unknown as DOMRectList;
  const emptyRect = () => new DOMRect(0, 0, 0, 0);
  if (typeof Range !== "undefined" && !Range.prototype.getClientRects) {
    Range.prototype.getClientRects = emptyRects;
    Range.prototype.getBoundingClientRect = emptyRect;
  }
  if (typeof Element !== "undefined" && !Element.prototype.getClientRects) {
    Element.prototype.getClientRects = emptyRects;
  }
  if (typeof document !== "undefined" && !document.elementFromPoint) {
    document.elementFromPoint = () => null;
  }
}
