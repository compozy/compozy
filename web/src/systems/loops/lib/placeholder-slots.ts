export interface PlaceholderSlot {
  /** Stable positional id, usable as a React key. */
  readonly id: string;
  readonly position: number;
}

/**
 * The repeated rows, cells or fields of a loading skeleton. A placeholder has no identity of its
 * own, so each slot carries a positional id instead of a raw index in the render callback.
 */
export function placeholderSlots(prefix: string, count: number): PlaceholderSlot[] {
  return Array.from({ length: count }, (_, position) => ({
    id: `${prefix}-${position}`,
    position,
  }));
}
