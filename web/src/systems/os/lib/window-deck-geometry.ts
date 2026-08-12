const EDGE_SCROLL_BAND = 24;
const EDGE_SCROLL_STEP = 12;

export function insertSlotAt(
  members: readonly string[],
  tabElements: ReadonlyMap<string, HTMLElement>,
  clientX: number
): number {
  let slot = 0;
  for (const member of members) {
    const element = tabElements.get(member);
    if (!element) continue;
    const box = element.getBoundingClientRect();
    if (clientX > box.left + box.width / 2) slot += 1;
  }
  return slot;
}

type HorizontalDeckBounds = Pick<DOMRect, "left" | "right">;
type DeckBounds = Pick<DOMRect, "bottom" | "left" | "right" | "top">;

export function autoScrollDeckAtEdges(
  tabs: HTMLElement,
  clientX: number,
  box: HorizontalDeckBounds = tabs.getBoundingClientRect()
): void {
  if (clientX < box.left + EDGE_SCROLL_BAND) tabs.scrollLeft -= EDGE_SCROLL_STEP;
  else if (clientX > box.right - EDGE_SCROLL_BAND) tabs.scrollLeft += EDGE_SCROLL_STEP;
}

export function pointInsideDeck(
  tabs: HTMLElement,
  point: { clientX: number; clientY: number },
  slop: { top: number; bottom: number },
  box: DeckBounds = tabs.getBoundingClientRect()
): boolean {
  return (
    point.clientX >= box.left &&
    point.clientX <= box.right &&
    point.clientY >= box.top - slop.top &&
    point.clientY <= box.bottom + slop.bottom
  );
}
