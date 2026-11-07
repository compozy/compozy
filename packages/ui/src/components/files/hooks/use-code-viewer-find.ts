"use client";

import * as React from "react";

/** One find match: a 1-based line and a half-open UTF-16 range within that line's text. */
export interface CodeViewerMatch {
  line: number;
  start: number;
  end: number;
}

type HighlightRegistry = Map<string, unknown>;
type HighlightConstructor = new (...ranges: Range[]) => unknown;

function highlightApi(): { registry: HighlightRegistry; Highlight: HighlightConstructor } | null {
  const css = globalThis.CSS as (typeof CSS & { highlights?: HighlightRegistry }) | undefined;
  const Highlight = (globalThis as { Highlight?: HighlightConstructor }).Highlight;
  if (!css?.highlights || !Highlight) return null;
  return { registry: css.highlights, Highlight };
}

/** A DOM range over `[start, end)` of a rendered line's text, across its token spans. */
function lineRange(line: Element, start: number, end: number): Range | null {
  const walker = line.ownerDocument.createTreeWalker(line, NodeFilter.SHOW_TEXT);
  const range = line.ownerDocument.createRange();
  let offset = 0;
  let started = false;
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    const length = node.nodeValue?.length ?? 0;
    if (!started && start < offset + length) {
      range.setStart(node, start - offset);
      started = true;
    }
    if (started && end <= offset + length) {
      range.setEnd(node, end - offset);
      return range;
    }
    offset += length;
  }
  return null;
}

/** Rendered content lines (`[data-line]`) inside the viewer's Pierre shadow roots. */
function renderedLines(container: HTMLElement): Element[] {
  const lines: Element[] = [];
  for (const host of container.querySelectorAll("diffs-container")) {
    const root = host.shadowRoot ?? host;
    lines.push(...root.querySelectorAll("[data-line]"));
  }
  return lines;
}

/**
 * Paints find matches over the virtualized lines with the CSS Custom Highlight API, so
 * Pierre's DOM is never rewritten. Only rendered lines get ranges; `sync` runs again after
 * every render or scroll. Without the API (older engines, jsdom) nothing is painted.
 */
export function useCodeViewerFind(matches: readonly CodeViewerMatch[], active: number | null) {
  const id = React.useId().replace(/[^a-zA-Z0-9]/g, "");
  const names = React.useMemo(
    () => ({ all: `cz-find-${id}`, active: `cz-find-active-${id}` }),
    [id]
  );
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const byLine = React.useMemo(() => {
    const map = new Map<number, { match: CodeViewerMatch; index: number }[]>();
    matches.forEach((match, index) => {
      const list = map.get(match.line);
      if (list) list.push({ match, index });
      else map.set(match.line, [{ match, index }]);
    });
    return map;
  }, [matches]);

  const sync = React.useCallback(() => {
    const api = highlightApi();
    const container = containerRef.current;
    if (!api) return;
    if (!container || byLine.size === 0) {
      api.registry.delete(names.all);
      api.registry.delete(names.active);
      return;
    }
    const all: Range[] = [];
    const current: Range[] = [];
    for (const line of renderedLines(container)) {
      const found = byLine.get(Number(line.getAttribute("data-line")));
      if (!found) continue;
      for (const { match, index } of found) {
        const range = lineRange(line, match.start, match.end);
        if (!range) continue;
        (index === active ? current : all).push(range);
      }
    }
    api.registry.set(names.all, new api.Highlight(...all));
    api.registry.set(names.active, new api.Highlight(...current));
  }, [active, byLine, names]);

  // `scheduleSync` stays stable, so Pierre's options never change with the matches.
  const syncRef = React.useRef(sync);
  React.useLayoutEffect(() => {
    syncRef.current = sync;
  });
  const frameRef = React.useRef<number | null>(null);
  const scheduleSync = React.useCallback(() => {
    if (frameRef.current !== null) return;
    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = null;
      syncRef.current();
    });
  }, []);

  // New matches or a new current match repaint on the next frame.
  React.useEffect(() => {
    const frame = requestAnimationFrame(sync);
    return () => cancelAnimationFrame(frame);
  }, [sync]);
  React.useEffect(
    () => () => {
      if (frameRef.current !== null) cancelAnimationFrame(frameRef.current);
      const api = highlightApi();
      api?.registry.delete(names.all);
      api?.registry.delete(names.active);
    },
    [names]
  );

  // Rules live in the shadow root; colours stay token variables inherited from the host.
  const unsafeCSS = `::highlight(${names.all}) { background-color: var(--color-brand-tint); }
::highlight(${names.active}) { background-color: var(--color-brand-tint-strong); text-decoration: underline 2px var(--color-brand); }`;

  return { containerRef, scheduleSync, unsafeCSS };
}
