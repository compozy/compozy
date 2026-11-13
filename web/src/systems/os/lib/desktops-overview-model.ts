import type { OsWindow } from "./os-types";
import type { LayoutNode, TabId, WindowId } from "./window-manager-types";

export interface DesktopOverviewTab {
  id: TabId;
  /** The member that names the tab and takes focus when it is chosen. */
  leadWindowId: WindowId;
  /** Members in reading order, lead first. */
  members: readonly OsWindow[];
}

export interface DesktopOverviewItem {
  id: string;
  name: string;
  /** The desktop's tabs in bar order. */
  tabs: readonly DesktopOverviewTab[];
  /**
   * Tabs closing the desktop moves elsewhere, counted on the daemon's tabs: every tab but one
   * holding a lone blank New Tab, including tabs whose app this web does not show.
   */
  movableTabCount: number;
  /** The tab this client shows on the desktop. */
  activeTabId: TabId | null;
  /** That tab's split tree, drawn as the card's thumbnail. */
  activeRoot: LayoutNode | null;
  /** A window on this desktop needs you. */
  needsYou?: boolean;
  /** The live switch chord for this position ("⌃1"), when bound. */
  switchShortcut?: string | null;
}

export type DesktopsOverviewState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "conflict"; message: string }
  | {
      status: "ready";
      desktops: readonly DesktopOverviewItem[];
      activeDesktopId: string | null;
    };

export interface DesktopOverviewFocusSegment {
  readonly direction: "earlier" | "later";
  readonly hiddenDesktopIds: readonly string[];
}

/** A desktop menu action the overview opens on: that card starts renaming, or closing. */
export interface DesktopOverviewAction {
  readonly desktopId: string;
  readonly action: "rename" | "close";
}
