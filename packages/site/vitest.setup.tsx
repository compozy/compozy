import { createElement, type ImgHTMLAttributes } from "react";
import { vi } from "vitest";

type MockImageProps = Omit<ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  src: string | { src: string };
  priority?: boolean;
  quality?: number;
  fill?: boolean;
  placeholder?: string;
  blurDataURL?: string;
  unoptimized?: boolean;
};

vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    priority: _priority,
    quality: _quality,
    fill: _fill,
    placeholder: _placeholder,
    blurDataURL: _blurDataURL,
    unoptimized: _unoptimized,
    ...props
  }: MockImageProps) => {
    const resolvedSrc = typeof src === "string" ? src : src.src;

    return createElement("img", { ...props, alt, src: resolvedSrc });
  },
}));

// jsdom has no media queries or intersection observer; the marketing motion helpers only need them to exist.
if (typeof window !== "undefined") {
  window.matchMedia ??= ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as typeof window.matchMedia;
}

globalThis.IntersectionObserver ??= class {
  readonly root = null;
  readonly rootMargin = "";
  readonly thresholds = [];
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
} as unknown as typeof IntersectionObserver;
