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
  const openExternal = (raw: string): void => {
    const target = safeExternalURL(raw);
    if (target) void shell.openExternal(target).catch(onError);
  };
  window.webContents.on("will-navigate", event => {
    const decision = classifyNavigation(event.url, allowedOrigin);
    if (decision === "allow") return;
    event.preventDefault();
    if (decision === "external") openExternal(event.url);
  });
  window.webContents.setWindowOpenHandler(details => {
    openExternal(details.url);
    return { action: "deny" };
  });
  if (app.isPackaged) {
    window.webContents.on("devtools-opened", () => window.webContents.closeDevTools());
  }
}
