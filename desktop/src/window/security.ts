import { type BrowserWindow, type Session, shell } from "electron";

import { classifyNavigation, safeExternalURL } from "./navigation-policy";

function normalizedOrigin(raw: string | undefined): string | null {
  if (!raw) return null;
  try {
    return new URL(raw).origin;
  } catch {
    return null;
  }
}

function allowsProductClipboardWrite(
  permission: string,
  requestingURL: string,
  isMainFrame: boolean,
  productOrigin: string | undefined
): boolean {
  return (
    isMainFrame &&
    permission === "clipboard-sanitized-write" &&
    normalizedOrigin(requestingURL) !== null &&
    normalizedOrigin(requestingURL) === normalizedOrigin(productOrigin)
  );
}

export function applyDefaultDenyPermissions(session: Session, productOrigin?: string): void {
  session.setPermissionCheckHandler((_webContents, permission, requestingOrigin, details) =>
    allowsProductClipboardWrite(permission, requestingOrigin, details.isMainFrame, productOrigin)
  );
  session.setPermissionRequestHandler((_webContents, permission, callback, details) =>
    callback(
      allowsProductClipboardWrite(
        permission,
        details.requestingUrl,
        details.isMainFrame,
        productOrigin
      )
    )
  );
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
}
