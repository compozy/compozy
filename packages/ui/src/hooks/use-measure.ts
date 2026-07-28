"use client";

import * as React from "react";

export interface MeasuredBounds {
  width: number;
  height: number;
}

export type UseMeasureResult<T extends HTMLElement = HTMLElement> = readonly [
  React.RefCallback<T>,
  MeasuredBounds,
];

function observeElement(node: Element, onBounds: (bounds: MeasuredBounds) => void): () => void {
  const observer = new ResizeObserver(entries => {
    const entry = entries[0];
    if (!entry) return;
    onBounds(entry.contentRect);
  });
  observer.observe(node);
  return () => observer.disconnect();
}

export function useMeasure<T extends HTMLElement = HTMLElement>(): UseMeasureResult<T> {
  const [bounds, setBounds] = React.useState<MeasuredBounds>({ width: 0, height: 0 });
  const [refCallback] = React.useState<React.RefCallback<T>>(() => (node: T | null) => {
    if (!node || typeof ResizeObserver === "undefined") {
      return;
    }
    return observeElement(node, ({ width, height }) => {
      setBounds(prev =>
        prev.width === width && prev.height === height ? prev : { width, height }
      );
    });
  });

  return [refCallback, bounds] as const;
}
