export function loadSessionWindowContent() {
  return import("./session-window-content");
}

export function loadSessionThread() {
  return import("@/components/assistant-ui/session-thread");
}

export function preloadSessionWindowModules(): Promise<unknown[]> {
  return Promise.all([loadSessionWindowContent(), loadSessionThread()]);
}
