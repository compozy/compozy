import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { ensureWindowSlotStore } from "../lib/window-slot-registry";
import { useWindowMemberSlot } from "./use-window-member-slot";

/** A window's title: a session's display title, the surface's live label, else the app's name. */
export function useOsWindowTitle(win: OsWindow, session: SessionPayload | undefined): string {
  const slot = useWindowMemberSlot(ensureWindowSlotStore(win.id));
  if (win.app === "session" && session) return getSessionDisplayTitle(session);
  const label = slot?.tabLabel ?? slot?.crumb;
  return typeof label === "string" && label.trim() !== ""
    ? label
    : getOsAppDescriptor(win.app).title;
}
