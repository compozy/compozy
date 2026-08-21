/**
 * Where a `Command` list's keyboard selection lands after the items underneath
 * it changed.
 *
 * A `Command` driven with `shouldFilter={false}` owns its own ordering, so it
 * also owns selection survival: cmdk re-highlights the first item whenever the
 * item set churns, which would move the operator's target out from under ⏎ every
 * time a live list refreshed. An item that survives the churn keeps the
 * selection; only a selection whose item actually disappeared moves, and then to
 * whatever now occupies its place — the nearest neighbour in both directions.
 *
 * Values are the stable item identities (`CommandItem value`), not labels.
 */
export function resolveCommandSelection(
  previous: readonly string[],
  next: readonly string[],
  selected: string
): string {
  if (next.includes(selected)) return selected;
  if (next.length === 0) return "";
  const index = previous.indexOf(selected);
  if (index < 0) return next[0] ?? "";
  return next[Math.min(index, next.length - 1)] ?? "";
}
