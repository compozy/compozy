/**
 * A divider position is a place people already have a word for. Snapping to the
 * halves, thirds and quarters — and naming them — is what keeps the canvas from
 * demanding the pixel precision the old decimal fields demanded.
 */
const NAMED_FRACTIONS: ReadonlyArray<readonly [number, string]> = [
  [0.25, "¼"],
  [1 / 3, "⅓"],
  [0.5, "½"],
  [2 / 3, "⅔"],
  [0.75, "¾"],
];

const INTERIOR_DETENTS = NAMED_FRACTIONS.map(([value]) => value);

export const LAYOUT_DETENT_TOLERANCE = 0.018;
export const LAYOUT_EDGE_DETENT_TOLERANCE = 0.014;

/** The nearest detent within tolerance, or null when the drag is between them. */
export function nearestDetent(
  value: number,
  tolerance: number = LAYOUT_DETENT_TOLERANCE
): number | null {
  for (const detent of INTERIOR_DETENTS) {
    if (Math.abs(value - detent) < tolerance) return detent;
  }
  return null;
}

/** Group edges may also land flush against the screen, which interior seams cannot. */
export function nearestEdgeDetent(
  value: number,
  tolerance: number = LAYOUT_EDGE_DETENT_TOLERANCE
): number | null {
  if (Math.abs(value) < tolerance) return 0;
  if (Math.abs(value - 1) < tolerance) return 1;
  return nearestDetent(value, tolerance);
}

/** "½" when it is one, "38%" when it is not. */
export function fractionLabel(value: number): string {
  for (const [fraction, label] of NAMED_FRACTIONS) {
    if (Math.abs(value - fraction) < 0.004) return label;
  }
  return `${Math.round(value * 100)}%`;
}
