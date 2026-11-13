import type {
  MoveWindowInput,
  OsArrangePreset,
  WindowManagerCommandOutcome,
} from "../lib/os-types";
import {
  arrangeLayoutCommand,
  balanceLayoutCommand,
  moveWindowCommand,
  resizeLayoutCommand,
} from "../lib/window-manager-command-builders";
import { arrangeParticipants } from "../lib/window-manager-navigation";
import { WindowManagerTabRuntime } from "./window-manager-tab-commands";

/**
 * Layout half of the command surface: moving windows between tabs and splits, arrange presets,
 * and split weights. Every method dispatches one semantic daemon command; pixels never persist.
 */
export abstract class WindowManagerLayoutRuntime extends WindowManagerTabRuntime {
  moveWindow = (id: string, input: MoveWindowInput): WindowManagerCommandOutcome =>
    this.dispatch(moveWindowCommand(id, input));

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
}
