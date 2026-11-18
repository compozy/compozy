import {
  useOsTabDragDesktopTarget,
  useOsTabDragSourceDesktop,
  useOsTabDragStart,
} from "./use-os-tab-drag";
import { useOsTabDragKind } from "./use-desktop-indicators-drag";

/**
 * The desktops overview's part in a tab drag: starting one from a card, its drop target, and what
 * the drag moves, named on the target card as the indicators name it.
 */
export function useDesktopOverviewDrag() {
  const { startOverviewTabDrag } = useOsTabDragStart();
  const dropTargetDesktopId = useOsTabDragDesktopTarget();
  // While a tab is dragged the overview is a drop surface: never modal, so it cannot trap focus,
  // make the dragged tab inert or take the release as an outside press.
  const dropSurface = useOsTabDragSourceDesktop() !== null;
  const dragKind = useOsTabDragKind();
  return { startOverviewTabDrag, dropTargetDesktopId, dropSurface, dragKind };
}
