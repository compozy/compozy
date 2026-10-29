/**
 * Maps between the prototype DOM (inside the canvas iframe) and comment targets: what was clicked
 * or dragged becomes an `ElementTarget` (selector, text, source location, rects), and a stored
 * target becomes a live rect for its pin. Coordinates are iframe viewport (canvas) pixels.
 */
import type { ElementTarget, Rect } from "#gallery/shell/comments/model";

const sourceAttribute = "data-proto-src";
const chainLimit = 8;

export type Size = { width: number; height: number };

const toRect = (box: DOMRect | Rect): Rect => ({
  x: box.x,
  y: box.y,
  width: box.width,
  height: box.height,
});

function segment(element: Element) {
  const testId = element.getAttribute("data-testid");
  if (testId) return `[data-testid="${CSS.escape(testId)}"]`;
  const tag = element.localName;
  const parent = element.parentElement;
  if (!parent) return tag;
  const siblings = [...parent.children].filter(child => child.localName === tag);
  return siblings.length > 1 ? `${tag}:nth-of-type(${siblings.indexOf(element) + 1})` : tag;
}

/** Shortest unique `a > b > c` path from the element up, anchored on `data-testid` when present. */
export function selectorFor(element: Element) {
  const doc = element.ownerDocument;
  const parts: string[] = [];
  let node: Element | null = element;
  while (node && node !== doc.documentElement) {
    parts.unshift(segment(node));
    const selector = parts.join(" > ");
    const matches = doc.querySelectorAll(selector);
    if (matches.length === 1 && matches[0] === element) return selector;
    node = node.parentElement;
  }
  return parts.join(" > ");
}

function textOf(element: Element) {
  const label =
    element.getAttribute("aria-label") ??
    element.getAttribute("placeholder") ??
    (element as HTMLElement).innerText ??
    element.textContent ??
    "";
  const text = label.replace(/\s+/g, " ").trim();
  return text.length > 120 ? `${text.slice(0, 119)}…` : text;
}

type FiberLike = { return: FiberLike | null; memoizedProps?: Record<string, unknown> | null };

/**
 * Source locations of the JSX elements that produced a DOM node, nearest first. It walks React's
 * fiber tree (dev build), so it also sees components that do not forward `data-proto-src` to the
 * DOM (a prototype's `<SessionBoardCard>` call); the DOM attributes are the fallback.
 */
export function sourceChain(element: Element, limit = chainLimit) {
  const chain: string[] = [];
  const add = (value: unknown) => {
    if (typeof value === "string" && !chain.includes(value)) chain.push(value);
  };
  const key = Object.keys(element).find(name => name.startsWith("__reactFiber$"));
  let fiber = key ? (element as unknown as Record<string, FiberLike | undefined>)[key] : undefined;
  while (fiber && chain.length < limit) {
    add(fiber.memoizedProps?.[sourceAttribute]);
    fiber = fiber.return ?? undefined;
  }
  let node: Element | null = element.closest(`[${sourceAttribute}]`);
  while (node && chain.length < limit) {
    add(node.getAttribute(sourceAttribute));
    node = node.parentElement?.closest(`[${sourceAttribute}]`) ?? null;
  }
  return chain;
}

/** Topmost prototype element at a canvas point (never the root `<html>`). */
export function elementAt(doc: Document, x: number, y: number) {
  const element = doc.elementFromPoint(x, y);
  return element && element !== doc.documentElement ? element : null;
}

/** Sources of what is drawn inside a region (a 3×3 grid of probes), most specific first. */
function regionSources(doc: Document, region: Rect) {
  const sources: string[] = [];
  for (const fy of [0.5, 0.2, 0.8]) {
    for (const fx of [0.5, 0.2, 0.8]) {
      const probe = elementAt(doc, region.x + region.width * fx, region.y + region.height * fy);
      const value = probe ? sourceChain(probe, 1)[0] : undefined;
      if (value && !sources.includes(value)) sources.push(value);
    }
  }
  return sources;
}

const contains = (outer: DOMRect, inner: Rect) =>
  outer.left <= inner.x + 1 &&
  outer.top <= inner.y + 1 &&
  outer.right >= inner.x + inner.width - 1 &&
  outer.bottom >= inner.y + inner.height - 1;

/** Deepest element that contains a dragged region, starting under its center. */
export function containerOf(doc: Document, region: Rect) {
  let node = elementAt(doc, region.x + region.width / 2, region.y + region.height / 2);
  while (node?.parentElement && !contains(node.getBoundingClientRect(), region)) {
    node = node.parentElement;
  }
  return node ?? doc.body;
}

const fraction = (value: number, total: number) => (total > 0 ? value / total : 0);

export function describeTarget(element: Element, region: Rect | null, canvas: Size): ElementTarget {
  const box = element.getBoundingClientRect();
  const rect = region ?? toRect(box);
  const inside = region ? regionSources(element.ownerDocument, region) : [];
  const chain = [...new Set([...inside, ...sourceChain(element)])].slice(0, chainLimit);
  return {
    kind: region ? "region" : "element",
    selector: selectorFor(element),
    testId: element.closest("[data-testid]")?.getAttribute("data-testid") ?? null,
    tag: element.localName,
    text: textOf(element),
    source: chain[0] ?? null,
    sourceChain: chain,
    rect,
    rectNormalized: {
      x: fraction(rect.x, canvas.width),
      y: fraction(rect.y, canvas.height),
      width: fraction(rect.width, canvas.width),
      height: fraction(rect.height, canvas.height),
    },
    offset: region
      ? {
          x: fraction(region.x - box.x, box.width),
          y: fraction(region.y - box.y, box.height),
          width: fraction(region.width, box.width),
          height: fraction(region.height, box.height),
        }
      : null,
  };
}

function anchorOf(doc: Document, target: ElementTarget) {
  try {
    const element = doc.querySelector(target.selector);
    if (element) return element;
  } catch {
    // A selector from an older DOM shape can be invalid; fall back to the source location.
  }
  return target.source
    ? doc.querySelector(`[${sourceAttribute}="${CSS.escape(target.source)}"]`)
    : null;
}

export type Located = { rect: Rect; anchored: boolean };

/**
 * Where a stored target is now: on its anchor element when it still exists (`anchored`), else on
 * the normalized rect recorded at capture time.
 */
export function locateTarget(doc: Document, target: ElementTarget, canvas: Size): Located {
  const box = anchorOf(doc, target)?.getBoundingClientRect();
  if (box && (box.width > 0 || box.height > 0)) {
    const offset = target.offset;
    const rect = offset
      ? {
          x: box.x + offset.x * box.width,
          y: box.y + offset.y * box.height,
          width: offset.width * box.width,
          height: offset.height * box.height,
        }
      : toRect(box);
    return { rect, anchored: true };
  }
  const normalized = target.rectNormalized;
  return {
    rect: {
      x: normalized.x * canvas.width,
      y: normalized.y * canvas.height,
      width: normalized.width * canvas.width,
      height: normalized.height * canvas.height,
    },
    anchored: false,
  };
}

/** Scrolls the nearest scrollable ancestor under the pointer (the overlay swallows wheel events). */
export function scrollAt(doc: Document, x: number, y: number, deltaX: number, deltaY: number) {
  const view = doc.defaultView;
  let node: Element | null = elementAt(doc, x, y);
  while (node && view) {
    const style = view.getComputedStyle(node);
    const scrollable =
      (/(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight) ||
      (/(auto|scroll)/.test(style.overflowX) && node.scrollWidth > node.clientWidth);
    if (scrollable) {
      node.scrollBy(deltaX, deltaY);
      return;
    }
    node = node.parentElement;
  }
  view?.scrollBy(deltaX, deltaY);
}
