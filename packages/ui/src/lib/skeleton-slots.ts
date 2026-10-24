export interface SkeletonSlot {
  /** Stable positional id, usable as a React key. */
  readonly id: string;
  readonly position: number;
}

/**
 * The repeated rows or cells of a skeleton. Placeholders have no identity of their own, so each
 * slot gets a positional id (not a raw array index in the render callback) and its position.
 */
export function skeletonSlots(prefix: string, count: number): SkeletonSlot[] {
  return Array.from({ length: count }, (_, position) => ({
    id: `${prefix}-${position}`,
    position,
  }));
}

/**
 * Deterministic variety: repeated placeholders cycle through `values` by position, so a stack of
 * rows does not read as one stamped block, and never depends on randomness (renders stay pure).
 */
export function cycleByPosition<T>(values: readonly T[], position: number): T {
  return values[position % values.length];
}
