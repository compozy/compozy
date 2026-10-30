/** The item before or after the current one, wrapping around; `null` with nothing else to reach. */
export function adjacentShortcutItem<T extends { id: string }>(
  items: readonly T[],
  currentId: string | null,
  direction: "previous" | "next"
): T | null {
  if (items.length < 2) return null;
  const currentIndex = items.findIndex(item => item.id === currentId);
  const start = currentIndex < 0 ? 0 : currentIndex;
  const offset = direction === "next" ? 1 : -1;
  return items[(start + offset + items.length) % items.length] ?? null;
}
