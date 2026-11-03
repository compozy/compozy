import { matchSessionInstance } from "./app-catalog";
import type { OsWindow } from "./os-types";

/** A window and the session it showed when it was inspected. */
export interface ShownSession {
  windowId: string;
  sessionId: string;
}

/** The owner profile id of a session, or `null` when it cannot be told (gone, unreadable). */
export type SessionOwnerLookup = (sessionId: string) => Promise<string | null>;

/**
 * The windows of a profile's desk that show another profile's session.
 *
 * Under All profiles the client works on the destination profile's desk, so a session of any
 * profile opened there lands in that desk. Once the operator switches to one profile, those
 * windows are not part of it. Only a confirmed foreign owner counts: a session whose owner cannot
 * be read stays, because the deletion reconciliation owns a session that is gone.
 */
export async function foreignProfileSessionWindows(
  windows: Readonly<Record<string, OsWindow>>,
  profileId: string,
  ownerOf: SessionOwnerLookup
): Promise<ShownSession[]> {
  const shown = Object.values(windows).flatMap(win => {
    const sessionId = shownSessionId(win);
    return sessionId === null ? [] : [{ windowId: win.id, sessionId }];
  });
  const owners = await Promise.all(
    shown.map(async ({ sessionId }) => {
      try {
        return await ownerOf(sessionId);
      } catch {
        return null;
      }
    })
  );
  return shown.filter((_, index) => {
    const owner = owners[index];
    return owner !== null && owner !== undefined && owner !== profileId;
  });
}

/** The session a window shows, or `null` for a window that shows none. */
export function shownSessionId(win: OsWindow | undefined): string | null {
  if (win === undefined) return null;
  try {
    return matchSessionInstance(win.route.pathname);
  } catch {
    // A malformed escape in one route must not abort the sweep of every other window.
    return null;
  }
}
