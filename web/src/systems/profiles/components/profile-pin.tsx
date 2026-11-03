import type { ReactNode } from "react";

import { ProfilePinContext } from "../lib/profile-pin-context";

/** Pins every profile-scoped read below to `profile`; a blank one follows the shell. */
export function ProfilePin({ profile, children }: { profile?: string; children: ReactNode }) {
  const pinned = profile?.trim() || null;
  return <ProfilePinContext value={pinned}>{children}</ProfilePinContext>;
}
