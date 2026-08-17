import type { LayoutDesktop } from "./window-manager-types";

/** Stable desktop order shared by directional and numeric switching. */
export function orderedDesktops(desktops: readonly LayoutDesktop[]): readonly LayoutDesktop[] {
  return [...desktops].sort(
    (left, right) => left.order - right.order || left.id.localeCompare(right.id)
  );
}
