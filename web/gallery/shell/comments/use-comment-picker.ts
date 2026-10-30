/**
 * Comment mode over the canvas: hovering highlights the element under the pointer, a click picks
 * it and a drag picks a region (its deepest containing element is the anchor). Esc drops the
 * draft first, then leaves comment mode. The wheel still scrolls the prototype.
 */
import { useEffect, useEffectEvent, useState, type PointerEvent, type WheelEvent } from "react";

import {
  containerOf,
  describeTarget,
  elementAt,
  scrollAt,
  type Size,
} from "#gallery/shell/comments/dom";
import type { ElementTarget, Rect } from "#gallery/shell/comments/model";

type Point = { x: number; y: number };

/** Below this many canvas px in both directions, a drag counts as a click. */
const clickSlop = 6;

export const dragRect = (a: Point, b: Point): Rect => ({
  x: Math.min(a.x, b.x),
  y: Math.min(a.y, b.y),
  width: Math.abs(a.x - b.x),
  height: Math.abs(a.y - b.y),
});

function release(event: PointerEvent<HTMLDivElement>) {
  if (event.currentTarget.hasPointerCapture(event.pointerId)) {
    event.currentTarget.releasePointerCapture(event.pointerId);
  }
}

type PickerOptions = {
  doc: Document | null;
  canvas: Size;
  scale: number;
  picking: boolean;
  onExit: () => void;
};

export function useCommentPicker({ doc, canvas, scale, picking, onExit }: PickerOptions) {
  const [hover, setHover] = useState<Rect | null>(null);
  const [drag, setDrag] = useState<{ start: Point; end: Point } | null>(null);
  const [draft, setDraft] = useState<ElementTarget | null>(null);

  const onEscape = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    setDrag(null);
    setHover(null);
    if (draft) setDraft(null);
    else onExit();
  });

  useEffect(() => {
    if (!picking) return;
    const listener = (event: KeyboardEvent) => onEscape(event);
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [picking]);

  const pointOf = (event: { clientX: number; clientY: number; currentTarget: HTMLDivElement }) => {
    const bounds = event.currentTarget.getBoundingClientRect();
    return { x: (event.clientX - bounds.left) / scale, y: (event.clientY - bounds.top) / scale };
  };

  const onPointerMove = (event: PointerEvent<HTMLDivElement>) => {
    if (!doc || !picking || draft) return;
    const point = pointOf(event);
    if (drag) {
      setDrag({ ...drag, end: point });
      return;
    }
    setHover(elementAt(doc, point.x, point.y)?.getBoundingClientRect() ?? null);
  };

  const onPointerDown = (event: PointerEvent<HTMLDivElement>) => {
    if (!doc || !picking || draft || event.button !== 0) return;
    event.currentTarget.setPointerCapture(event.pointerId);
    const point = pointOf(event);
    setDrag({ start: point, end: point });
  };

  const onPointerUp = (event: PointerEvent<HTMLDivElement>) => {
    release(event);
    if (!doc || !drag) return;
    const region = dragRect(drag.start, pointOf(event));
    setDrag(null);
    setHover(null);
    if (region.width < clickSlop && region.height < clickSlop) {
      const element = elementAt(doc, drag.start.x, drag.start.y);
      if (element) setDraft(describeTarget(element, null, canvas));
      return;
    }
    setDraft(describeTarget(containerOf(doc, region), region, canvas));
  };

  const onPointerCancel = (event: PointerEvent<HTMLDivElement>) => {
    release(event);
    setDrag(null);
    setHover(null);
  };

  const onWheel = (event: WheelEvent<HTMLDivElement>) => {
    if (!doc || !picking) return;
    const point = pointOf(event);
    scrollAt(doc, point.x, point.y, event.deltaX, event.deltaY);
  };

  return {
    hover,
    drag: drag ? dragRect(drag.start, drag.end) : null,
    draft,
    clearDraft: () => setDraft(null),
    handlers: {
      onPointerDown,
      onPointerMove,
      onPointerUp,
      onPointerCancel,
      onLostPointerCapture: onPointerCancel,
      onPointerLeave: () => setHover(null),
      onWheel,
    },
  };
}
