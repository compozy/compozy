import { notifyUser } from "@/lib/user-feedback";

import type { OsOpenTarget, WindowManagerCommandOutcome } from "./os-types";

/**
 * Explicit launch destinations (shell-rail P6). Plain activation stays
 * focus-first and lets the daemon's new-window policy — a tab in the focused
 * window by default — place whatever it has to open; these always create a
 * new instance where the operator asked for it.
 */
export type LaunchPlacement = "tab" | "split" | "window" | "desktop";

export interface LaunchModifiers {
  altKey: boolean;
  shiftKey: boolean;
}

/** Rail grammar: ⇧ opens on a new desktop, ⌥ splits beside focus; neither is plain activation. */
export function launchPlacementForModifiers(modifiers?: LaunchModifiers): LaunchPlacement | null {
  if (modifiers?.shiftKey) return "desktop";
  if (modifiers?.altKey) return "split";
  return null;
}

export interface LaunchPorts {
  manager: {
    getState(): { focusedId: string | null };
    createDesktop(desktopId: string): WindowManagerCommandOutcome;
    switchDesktop(desktopId: string): void;
  };
  coordinator: { userOpen(target: OsOpenTarget): Promise<string | null> };
}

/** Matches the daemon's `desktop-<32 hex>` generator, so client-created desktops look native. */
export function randomOsDesktopId(): string {
  const bytes = new Uint8Array(16);
  globalThis.crypto.getRandomValues(bytes);
  return `desktop-${Array.from(bytes, byte => byte.toString(16).padStart(2, "0")).join("")}`;
}

/** Opens a new instance of `target` at an explicit destination; resolves its window ID. */
export async function launchInPlacement(
  ports: LaunchPorts,
  target: OsOpenTarget,
  placement: LaunchPlacement
): Promise<string | null> {
  const fresh: OsOpenTarget = { ...target, forceNewInstance: true };
  switch (placement) {
    case "tab": {
      // Named explicitly so the tab holds under any configured policy; with
      // nothing focused the daemon places the window on its own.
      const focusedId = ports.manager.getState().focusedId;
      return ports.coordinator.userOpen(
        focusedId === null ? fresh : { ...fresh, stackTargetWindowId: focusedId }
      );
    }
    case "split":
      return ports.coordinator.userOpen({ ...fresh, placement: "split" });
    case "window":
      return ports.coordinator.userOpen({ ...fresh, placement: "floating" });
    case "desktop": {
      // Commands run in order, so the open lands after the switch: the empty
      // desktop gives the window the whole panel and the client its focus.
      const desktopId = randomOsDesktopId();
      if (!(await ports.manager.createDesktop(desktopId).completion)) {
        notifyUser({ message: "Couldn't create a new desktop. Try again.", tone: "error" });
        return null;
      }
      ports.manager.switchDesktop(desktopId);
      return ports.coordinator.userOpen({ ...fresh, desktopId });
    }
  }
}
