import {
  useOsTabDragDesktopTarget,
  useOsTabDragSourceDesktop,
  useOsTabDragStart,
} from "./use-os-tab-drag";

/** The desktops overview's part in a tab drag: starting one from a card, and its drop target. */
export function useDesktopOverviewDrag() {
  const { startOverviewTabDrag } = useOsTabDragStart();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  // While a tab is dragged the overview is a drop surface: never modal, so it cannot trap focus,
  // make the dragged tab inert or take the release as an outside press.
  const dropSurface = useOsTabDragSourceDesktop() !== null;
  return { startOverviewTabDrag, dropTargetDesktopId, dropSurface };
}
