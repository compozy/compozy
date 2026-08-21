import type { ProfilePayload, ProfileView } from "../types";

export const PERMANENT_PROFILE = "default";

export type ProfileRowState = "current" | "active" | "archived" | "needs-setup";

export interface ProfileRow {
  name: string;
  color: string;
  state: ProfileRowState;
  archived: boolean;
  current: boolean;
  needsSetup: boolean;
  permanent: boolean;
  workItems: number;
  /** Typed reason this row cannot be switched into, or "" when it can. */
  disabledReason: string;
}

export function isArchived(profile: ProfilePayload): boolean {
  return profile.state === "archived";
}

/**
 * Why a row cannot be selected, in the same words the canonical surfaces use.
 *
 * Kept here so the switcher, Settings, and the palette cannot drift into three
 * different explanations of the same refusal (US-034.EC-2).
 */
function disabledReasonFor(profile: ProfilePayload): string {
  if (isArchived(profile)) return "archived";
  if (profile.needs_setup === true) return "needs setup";
  return "";
}

function stateFor(profile: ProfilePayload, current: boolean): ProfileRowState {
  if (isArchived(profile)) return "archived";
  if (profile.needs_setup === true) return "needs-setup";
  return current ? "current" : "active";
}

export function toProfileRow(profile: ProfilePayload, currentName: string): ProfileRow {
  const current = profile.name === currentName;
  return {
    name: profile.name,
    color: profile.color,
    state: stateFor(profile, current),
    archived: isArchived(profile),
    current,
    needsSetup: profile.needs_setup === true,
    permanent: profile.name === PERMANENT_PROFILE,
    workItems: profile.work_items ?? 0,
    disabledReason: disabledReasonFor(profile),
  };
}

export function toProfileRows(
  profiles: readonly ProfilePayload[],
  currentName: string
): ProfileRow[] {
  return profiles.map(profile => toProfileRow(profile, currentName));
}

export function activeProfiles(profiles: readonly ProfilePayload[]): ProfilePayload[] {
  return profiles.filter(profile => !isArchived(profile));
}

export function archivedProfiles(profiles: readonly ProfilePayload[]): ProfilePayload[] {
  return profiles.filter(isArchived);
}

/**
 * Delete is offered only where it can actually succeed: an archived profile that
 * owns nothing (US-006.AC-2). Everywhere else the flow routes to archive, so a
 * button that would always refuse is simply absent.
 */
export function canDelete(profile: ProfilePayload): boolean {
  return (
    isArchived(profile) && (profile.work_items ?? 0) === 0 && profile.name !== PERMANENT_PROFILE
  );
}

/** The switcher stays quiet until a second profile exists (US-007.EC-2). */
export function isQuiet(profiles: readonly ProfilePayload[]): boolean {
  return activeProfiles(profiles).length <= 1;
}

export function viewLabel(view: ProfileView, aggregateLabel: string): string {
  return view.kind === "aggregate" ? aggregateLabel : view.profile;
}

/** The profile a view acts as. The aggregate always creates into `default` (ADR-005). */
export function actingProfile(view: ProfileView): string {
  return view.kind === "aggregate" ? PERMANENT_PROFILE : view.profile;
}
