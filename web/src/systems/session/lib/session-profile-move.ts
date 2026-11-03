import type { SessionPayload } from "../types";

/**
 * Why the daemon would refuse moving this session to another profile, as far as
 * the row knows; `null` when the move can be tried. The daemon still decides:
 * a worktree with uncommitted changes is only known there.
 */
export function sessionProfileMoveRefusal(session: SessionPayload): string | null {
  if (session.state !== "stopped") return "Stop this session before moving it.";
  if ((session.children?.total ?? 0) > 0) {
    return "This session has child sessions. Make them top-level first.";
  }
  return null;
}
