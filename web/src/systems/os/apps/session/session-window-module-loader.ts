export function loadSessionWindowContent() {
  return import("./session-window-content");
}

export function loadSessionThread() {
  return import("@/components/assistant-ui/session-thread");
}

export function loadSessionInspector() {
  return import("@/systems/session/components/session-inspector");
}

export function preloadSessionWindowModules(): Promise<unknown[]> {
  // The inspector is opened by the operator, but a rail that first lands after a chunk request
  // would open as a gap, so it is warmed with the surfaces the window is always built from.
  return Promise.all([loadSessionWindowContent(), loadSessionThread(), loadSessionInspector()]);
}
