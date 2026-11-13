import { orderedDesktops } from "../lib/desktop-order";
import type { WindowManagerCommandOutcome } from "../lib/os-types";
import { moveWindowCommand } from "../lib/window-manager-command-builders";
import { windowManagerStore } from "../stores/window-manager-store";
import { WindowManagerLayoutRuntime } from "./window-manager-layout-commands";

/** Desktop lifecycle commands shared by the window-manager runtime. */
export abstract class WindowManagerDesktopRuntime extends WindowManagerLayoutRuntime {
  createDesktop(desktopId = ""): WindowManagerCommandOutcome {
    return this.dispatch({
      commandId: "desktop.create",
      payload: { desktop_id: desktopId, name: "" },
    });
  }

  renameDesktop(desktopId: string, name: string): void {
    this.dispatch({ commandId: "desktop.update", payload: { desktop_id: desktopId, name } });
  }

  reorderDesktop(desktopId: string, order: number): void {
    this.dispatch({ commandId: "desktop.reorder", payload: { desktop_id: desktopId, order } });
  }

  switchDesktop(desktopId: string): void {
    const current = this.view.activeDesktopId;
    const config = this.view.windowManagerConfig;
    const accepted = this.dispatch({
      commandId: "desktop.switch",
      payload: { desktop_id: desktopId },
    });
    if (accepted.accepted && current !== null && current !== desktopId && config !== null) {
      windowManagerStore.trigger.transitionIntentChanged({
        intent: {
          fromDesktopId: current,
          toDesktopId: desktopId,
          direction: this.desktopTransitionDirection(this.view.desktops, current, desktopId),
          mode: this.reduceMotion ? "instant" : config.desktopTransition,
        },
      });
    }
  }

  switchDesktopDirection(direction: "previous" | "next"): void {
    const active = this.view.activeDesktopId;
    const ordered = orderedDesktops(this.view.desktops);
    const index = ordered.findIndex(desktop => desktop.id === active);
    if (index < 0) return;
    const target = ordered[index + (direction === "next" ? 1 : -1)];
    if (target) this.switchDesktop(target.id);
  }

  deleteDesktop(desktopId: string, destinationId: string | null): void {
    this.dispatch({
      commandId: "desktop.delete",
      payload: {
        desktop_id: desktopId,
        ...(destinationId ? { destination_id: destinationId } : {}),
      },
    });
  }

  /** The window's whole tab, split included, joins the end of the destination desktop's bar. */
  moveWindowToDesktop(windowId: string, destinationDesktopId: string): void {
    this.dispatch(
      moveWindowCommand(windowId, { destinationDesktopId, placement: "tab", moveGroup: true })
    );
  }
}
