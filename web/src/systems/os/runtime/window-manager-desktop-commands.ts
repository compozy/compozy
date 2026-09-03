import { orderedDesktops } from "../lib/desktop-order";
import { windowManagerStore } from "../stores/window-manager-store";
import { WindowManagerSnapRuntime } from "./window-manager-snap-commands";

/** Desktop lifecycle commands shared by the geometry runtime. */
export abstract class WindowManagerDesktopRuntime extends WindowManagerSnapRuntime {
  createDesktop(): void {
    this.dispatch({
      commandId: "desktop.create",
      payload: { desktop_id: "", name: "" },
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

  moveWindowToDesktop(windowId: string, destinationDesktopId: string): void {
    this.dispatch({
      commandId: "window.move",
      payload: {
        window_id: windowId,
        destination_desktop_id: destinationDesktopId,
        placement: "floating",
        move_group: false,
      },
    });
  }
}
