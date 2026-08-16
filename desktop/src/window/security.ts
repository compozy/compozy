import { app, type BrowserWindow, type Session, shell } from "electron";

import { classifyNavigation, safeExternalURL } from "./navigation-policy";

export function applyDefaultDenyPermissions(session: Session): void {
  session.setPermissionCheckHandler(() => false);
  session.setPermissionRequestHandler((_webContents, _permission, callback) => callback(false));
}

export function guardWindowNavigation(
  window: BrowserWindow,
  allowedOrigin: string,
  onError: (error: Error) => void
): void {
  window.webContents.on("will-navigate", event => {
    const decision = classifyNavigation(event.url, allowedOrigin);
    if (decision === "allow") return;
    event.preventDefault();
    if (decision === "external") {
      const target = safeExternalURL(event.url);
      if (target) void shell.openExternal(target).catch(onError);
    }
  });
  window.webContents.setWindowOpenHandler(details => {
    const target = safeExternalURL(details.url);
    if (target) void shell.openExternal(target).catch(onError);
    return { action: "deny" };
  });
  if (app.isPackaged) {
    window.webContents.on("devtools-opened", () => window.webContents.closeDevTools());
  }
}
