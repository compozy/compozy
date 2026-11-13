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
import type { LayoutNodeId, WindowManagerCommandInput } from "../lib/window-manager-types";
import { authoritativeTabBalance } from "../lib/window-manager-view";
import type { WindowManagerRuntimeBinding } from "./window-manager-runtime-core";
import { WindowManagerTabRuntime } from "./window-manager-tab-commands";

/** This client's commands in flight of one kind, and the waiters for none to be left. */
class InFlightCommands {
  private count = 0;
  private waiters: Array<() => void> = [];

  get busy(): boolean {
    return this.count > 0;
  }

  /** Counts a dispatched command until its completion settles. */
  track(completion: Promise<boolean>): void {
    this.count += 1;
    const settle = () => {
      this.count -= 1;
      if (this.count > 0) return;
      for (const resolve of this.waiters.splice(0)) resolve();
    };
    void completion.then(settle, settle);
  }

  /** Resolves once none is in flight: at once when none is. */
  settled(): Promise<void> {
    if (this.count === 0) return Promise.resolve();
    return new Promise(resolve => this.waiters.push(resolve));
  }
}

/**
 * Layout half of the command surface: moving windows between tabs and splits, arrange presets,
 * and split weights. Every method dispatches one semantic daemon command; pixels never persist.
 */
export abstract class WindowManagerLayoutRuntime extends WindowManagerTabRuntime {
  /** This client's commands that can move the layout, dispatched and not yet settled. */
  private readonly layoutInFlight = new InFlightCommands();
  /** Every command of this client dispatched and not yet settled, focus included. */
  private readonly allInFlight = new InFlightCommands();

  /**
   * Whether a command of this client that can move the layout is dispatched and not yet settled.
   * `window.focus` is not counted: it never moves a split, and a focus that shows another tab
   * changes the shown frame, which ends a seam gesture anyway. The runtime publishes a command's
   * snapshot before its completion settles, so its result lands while this holds.
   */
  commandsInFlight = (): boolean => this.layoutInFlight.busy;

  /** Resolves once no such command of this client is in flight: at once when none is. */
  commandsSettled = (): Promise<void> => this.layoutInFlight.settled();

  protected override dispatch(command: WindowManagerCommandInput): WindowManagerCommandOutcome {
    const outcome = super.dispatch(command);
    if (!outcome.accepted) return outcome;
    this.allInFlight.track(outcome.completion);
    if (command.commandId !== "window.focus") this.layoutInFlight.track(outcome.completion);
    return outcome;
  }

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
    delta: number,
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome =>
    this.dispatch({ ...resizeLayoutCommand(splitId, boundaryIndex, delta), ...fence });

  balanceLayout = (
    target: { tabId?: string; splitId?: string },
    fence?: WindowManagerCommandFence
  ): WindowManagerCommandOutcome => {
    const command = balanceLayoutCommand(target);
    if (command === null) return { accepted: false, completion: Promise.resolve(false) };
    return this.dispatch({ ...command, ...fence });
  };

  /**
   * "Balance sizes" for a whole tab, admitted like a seam double-click (`planTabBalance`): one
   * `layout.balance` of the tab when every split, evened together, still lays out side by side
   * wherever it does now; otherwise the splits that can be evened without that, outermost first;
   * nothing when none can. The plan is made against the latest layout and work area once every
   * command of this client in flight (focus included) has settled, so the balance is the next
   * command the runtime runs, and made again before every further split, so a layout or viewport
   * change while a balance waits never collapses a split.
   */
  balanceTab = (tabId: string): void => {
    this.balanceTabFrom(tabId, this.binding, new Set());
  };

  private balanceTabFrom(
    tabId: string,
    binding: WindowManagerRuntimeBinding | null,
    balanced: Set<LayoutNodeId>
  ): void {
    if (this.allInFlight.busy) {
      void this.allInFlight.settled().then(() => this.balanceTabFrom(tabId, binding, balanced));
      return;
    }
    if (this.binding !== binding) return;
    const plan = authoritativeTabBalance(this.view, this.workArea(), tabId);
    if (plan === null) return;
    if (plan.whole) {
      this.balanceLayout({ tabId });
      return;
    }
    const splitId = plan.splitIds.find(id => !balanced.has(id));
    if (splitId === undefined) return;
    balanced.add(splitId);
    void this.balanceLayout({ splitId }).completion.then(applied => {
      if (applied) this.balanceTabFrom(tabId, binding, balanced);
    });
  }

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
