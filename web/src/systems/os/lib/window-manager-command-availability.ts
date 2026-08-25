import type { OsDesktopRuntimeStore } from "./os-types";

/** Commands use HTTP and remain available while the event stream reconnects. */
export function windowManagerCommandsAvailable(state: OsDesktopRuntimeStore): boolean {
  return (
    state.snapshot !== null &&
    state.client !== null &&
    state.windowManagerConfig !== null &&
    state.hydration === "live"
  );
}
