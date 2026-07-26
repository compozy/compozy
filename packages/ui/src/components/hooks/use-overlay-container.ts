"use client";

import * as React from "react";

/**
 * OverlayContainerContext scopes where modal overlays portal.
 *
 * An `OsWindow` (or any bounded surface) provides the element overlays should
 * mount into so a dialog/sheet opened from window content renders inside that
 * window — dimming and traveling with it — instead of covering the whole
 * desktop. Outside a provider the value is `null`, and the `@agh/ui`
 * Dialog/Sheet portal wrappers fall back to their default `document.body`
 * mount, so existing call sites behave exactly as before.
 *
 * An explicit `container` prop on a portal wrapper always wins over context.
 */
export type OverlayContainer = HTMLElement | null;

export const OverlayContainerContext = React.createContext<OverlayContainer>(null);

export function useOverlayContainer(): OverlayContainer {
  return React.use(OverlayContainerContext);
}
