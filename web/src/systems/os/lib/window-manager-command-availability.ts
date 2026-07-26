import type { OsDesktopRuntimeStore } from "./os-types";

/** Commands require an authoritative snapshot and the registered live client fence. */
export function windowManagerCommandsAvailable(state: OsDesktopRuntimeStore): boolean {
  return (
    state.snapshot !== null &&
    state.client !== null &&
    state.hydration === "live" &&
    state.connectionStatus === "connected"
  );
}
