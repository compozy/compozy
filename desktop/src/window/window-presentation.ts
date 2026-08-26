import { app, type BrowserWindow } from "electron";

export type WindowPresentation = "foreground" | "inactive";

export function presentWindow(window: BrowserWindow, presentation: WindowPresentation): void {
  if (presentation === "inactive") {
    window.showInactive();
    return;
  }
  window.show();
  app.focus({ steal: true });
  window.moveTop();
  window.focus();
}
