import { findFoldedOccurrences } from "@/systems/session/lib/session-find-text";

/*
 * DOM ranges for find (S8): every occurrence of the committed query inside the
 * rendered rows, cut in the original text with the daemon's folding, plus the
 * one range a jump lands on. Pure DOM reads; nothing here mutates React's tree.
 */

function textNodeRanges(node: Text, needle: string): Range[] {
  return findFoldedOccurrences(node.data, needle).map(span => {
    const range = new Range();
    range.setStart(node, span.start);
    range.setEnd(node, span.end);
    return range;
  });
}

function* textNodes(scope: Node): Generator<Text> {
  const walker = document.createTreeWalker(scope, NodeFilter.SHOW_TEXT);
  for (let node = walker.nextNode(); node; node = walker.nextNode()) {
    yield node as Text;
  }
}

export interface FindRanges {
  matches: Range[];
  active: Range | null;
}

/** All occurrences under `root`'s message rows; the first inside the active row is the active mark. */
export function collectFindRanges(
  root: ParentNode,
  needle: string,
  activeMessageId: string | null
): FindRanges {
  const matches: Range[] = [];
  let active: Range | null = null;
  for (const row of root.querySelectorAll<HTMLElement>("[data-message-id]")) {
    const isActiveRow = activeMessageId !== null && row.dataset.messageId === activeMessageId;
    for (const node of textNodes(row)) {
      const ranges = textNodeRanges(node, needle);
      if (isActiveRow && active === null && ranges.length > 0) active = ranges[0]!;
      matches.push(...ranges);
    }
  }
  return { active, matches };
}

/** The first occurrence of `needle` under `scope`, or `null`. */
export function firstFindRange(scope: Node, needle: string): Range | null {
  if (needle.length === 0) return null;
  for (const node of textNodes(scope)) {
    const ranges = textNodeRanges(node, needle);
    if (ranges.length > 0) return ranges[0]!;
  }
  return null;
}

/**
 * Where a jump lands inside a message row: the matched text inside the matched
 * part when both are on screen, else the part itself, else the first
 * occurrence anywhere in the row, else nothing (the row's top edge lands).
 */
export function locateFindTarget(
  row: HTMLElement,
  partIndex: number | null,
  needle: string
): Range | null {
  const part =
    partIndex === null
      ? null
      : row.querySelector<HTMLElement>(
          `[data-part-index="${partIndex}"], [data-part-indices~="${partIndex}"]`
        );
  if (part) {
    const inPart = firstFindRange(part, needle);
    if (inPart) return inPart;
    const whole = new Range();
    whole.selectNodeContents(part);
    return whole;
  }
  return firstFindRange(row, needle);
}
