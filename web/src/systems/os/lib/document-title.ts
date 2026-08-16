/**
 * The tab / app title badge — the one attention channel that needs no
 * permission prompt and works while the app is backgrounded.
 *
 * The base title is captured once, on first write, so the count is applied over
 * whatever the document declares rather than a hardcoded string. Nothing else
 * in the app writes `document.title`, so the count survives route changes by
 * construction.
 *
 * The menubar pill caps at 9+; the title deliberately does not. A backgrounded
 * operator wants the real number.
 */

let baseTitle: string | null = null;

function resolveBaseTitle(): string {
  if (baseTitle === null) baseTitle = document.title;
  return baseTitle;
}

export function formatTitleBadge(base: string, count: number): string {
  return count > 0 ? `(${count}) ${base}` : base;
}

export function applyTitleBadge(count: number): void {
  if (typeof document === "undefined") return;
  const next = formatTitleBadge(resolveBaseTitle(), count);
  if (document.title !== next) document.title = next;
}

/** Restores the clean title. Used on teardown so a stale count never lingers. */
export function resetTitleBadge(): void {
  if (typeof document === "undefined" || baseTitle === null) return;
  if (document.title !== baseTitle) document.title = baseTitle;
}

/** Test seam: forget the captured base title between cases. */
export function resetDocumentTitleBase(): void {
  baseTitle = null;
}
