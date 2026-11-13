import type {
  MoveWindowInput,
  OsArrangePreset,
  WindowManagerCommandFence,
  WindowManagerCommandOutcome,
} from "../lib/os-types";
import type { DropEdge } from "../lib/drop-edge";
import { leadWindowId } from "../lib/tab-model";
import {
  adjacentTabIndex,
  arrangeLayoutCommand,
  balanceLayoutCommand,
  moveWindowCommand,
  reorderTabCommand,
  resizeLayoutCommand,
  splitWithCommand,
  swapWindowsCommand,
  tileWindowCommand,
  unsplitWindowCommand,
} from "../lib/window-manager-command-builders";
import { arrangeParticipants } from "../lib/window-manager-navigation";
import { WindowManagerTabRuntime } from "./window-manager-tab-commands";

/**
 * Layout half of the command surface: moving windows between tabs and splits, arrange presets,
 * and split weights. Every method dispatches one semantic daemon command; pixels never persist.
 */
export abstract class WindowManagerLayoutRuntime extends WindowManagerTabRuntime {
  /** A fenced move is refused when the window or its target is no longer where the gesture saw it. */
  moveWindow = (
    id: string,
    input: MoveWindowInput,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome => this.dispatch({ ...moveWindowCommand(id, input), ...fence });

  swapWindows = (
    firstWindowId: string,
    secondWindowId: string,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome =>
    this.dispatch({ ...swapWindowsCommand(firstWindowId, secondWindowId), ...fence });

  /** Gathers the anchor, the rest of its tab, and one window per other tab into one split tab. */
  arrangeLayout = (anchorId: string, preset: OsArrangePreset): void => {
    const anchor = this.view.windows[anchorId];
    const desktop = this.view.desktops.find(candidate => candidate.id === anchor?.desktopId);
    if (!anchor || !desktop) return;
    const participants = arrangeParticipants(
      desktop,
      anchorId,
      this.view.client?.focusOrder ?? [],
      preset
    );
    const command = arrangeLayoutCommand(desktop.id, participants, preset);
    if (command !== null) this.dispatch(command);
  };

  resizeLayout = (
    splitId: string,
    boundaryIndex: number,
    delta: number
  ): WindowManagerCommandOutcome =>
    this.dispatch(resizeLayoutCommand(splitId, boundaryIndex, delta));

  balanceLayout = (target: { tabId?: string; splitId?: string }): void => {
    const command = balanceLayoutCommand(target);
    if (command !== null) this.dispatch(command);
  };

  /** "Move to new tab": a split member becomes its own tab right after its source tab. */
  unsplitWindow = (windowId: string): void => {
    const window = this.view.windows[windowId];
    const command = window ? unsplitWindowCommand(window) : null;
    if (command !== null) this.dispatch(command);
  };

  /** Moves a split member to a root edge of its own tab (`window.tile.*`). */
  tileWindow = (windowId: string, edge: DropEdge): void => {
    const window = this.view.windows[windowId];
    const command = window ? tileWindowCommand(window, edge) : null;
    if (command !== null) this.dispatch(command);
  };

  /** Joins another tab of the same desktop to `hostTabId` as a split, on the right. */
  splitWithTab = (hostTabId: string, chosenTabId: string): void => {
    const desktop = this.view.desktops.find(candidate =>
      candidate.tabs.some(tab => tab.id === hostTabId)
    );
    const chosen = desktop?.tabs.find(tab => tab.id === chosenTabId);
    if (desktop === undefined || chosen === undefined || chosenTabId === hostTabId) return;
    const lead = leadWindowId(chosen, this.view.client?.focusOrder ?? []);
    if (lead !== null) this.dispatch(splitWithCommand(desktop.id, hostTabId, lead));
  };

  /** "Move left" / "Move right": the window's tab moves one step within its pinned range. */
  moveTab = (windowId: string, direction: "left" | "right"): void => {
    const window = this.view.windows[windowId];
    const desktop = this.view.desktops.find(candidate => candidate.id === window?.desktopId);
    if (window === undefined || desktop === undefined) return;
    const index = adjacentTabIndex(desktop.tabs, window.tabId, direction);
    if (index !== null) this.dispatch(reorderTabCommand(windowId, index));
  };
}
