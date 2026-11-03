import { useContext } from "react";

import { ProfilePinContext } from "../lib/profile-pin-context";
import type { ProfileView } from "../types";

/** The profile the enclosing window is pinned to, or `null` when it follows the shell. */
export function usePinnedProfile(): string | null {
  return useContext(ProfilePinContext);
}

/** The shell's view, unless the enclosing window is pinned to one profile. */
export function usePinnedProfileView(view: ProfileView): ProfileView {
  const pinned = usePinnedProfile();
  return pinned ? { kind: "profile", profile: pinned } : view;
}
