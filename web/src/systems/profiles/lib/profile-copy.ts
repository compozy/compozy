/**
 * Product language for Profiles.
 *
 * The two sentences below are the locked answers to the two questions operators
 * actually ask — what separates, and whether this is security. They are quoted
 * verbatim by the switcher menu, the Settings page, and the create dialog, so
 * every surface gives the same answer. The UI word for a workspace is
 * "project"; the runtime is "CompozyOS", never "the daemon".
 */
export const PROFILE_BOUNDARY_ANSWER =
  "Work is separate per profile. Project folders and machine tools are shared.";

export const PROFILE_SEPARATION_LINE =
  "Profiles keep work separate. They are not a security boundary.";

export const PROFILE_REMOTE_MANAGEMENT_LINE = "Profiles are managed on the machine that owns them.";

export const PROFILE_PERMANENT_LINE =
  "default is permanent. It cannot be renamed or archived. Create a profile for work you want to label.";

/**
 * Messages for the typed refusals in the developer-experience contract.
 *
 * Only the codes a web surface can actually provoke are mapped; anything else
 * falls through to the daemon's own message rather than being guessed at.
 */
const CODE_MESSAGES: Record<string, string> = {
  profile_name_invalid: "Use letters, numbers, spaces, or dashes.",
  profile_name_reserved: "That name is reserved. Pick another.",
  profile_permanent: PROFILE_PERMANENT_LINE,
  profile_plan_stale: "Something changed while this was open. Review the updated plan.",
  profile_remote_management_forbidden: PROFILE_REMOTE_MANAGEMENT_LINE,
  profile_archived: "That profile is archived. Unarchive it first.",
  profile_not_found: "That profile no longer exists.",
  profile_unavailable: "That profile is still finishing an earlier change.",
};

/** Inline message for a refusal, preferring our wording only where we have it. */
export function profileErrorMessage(code: string, fallback: string): string {
  return CODE_MESSAGES[code] ?? fallback;
}

export function nameTakenMessage(name: string): string {
  return `A profile named ${name} already exists.`;
}

export const NAME_REQUIRED_MESSAGE = "Give the profile a name.";

/** "3 sessions · 1 automation" — counts are data, never prose. */
export function workItemsLabel(count: number): string {
  if (count === 0) return "Empty";
  return count === 1 ? "1 item" : `${count} items`;
}

export function archivedFallbackToast(archived: string): string {
  return `${archived} was archived. You're back on default.`;
}

/**
 * The aggregate's own label. It matches the daemon's reserved wording exactly, so
 * the same two words name the state in the switcher, the palette footer, and the
 * lens the runtime reports back.
 */
export const ALL_PROFILES_LABEL = "All profiles";

/** Appended to an owner tag whose profile has since been archived. */
export const ARCHIVED_OWNER_SUFFIX = " · archived";

/** The confirmation after creating something while the aggregate is on. */
export function createdInProfileToast(profile: string): string {
  return `Created in ${profile}.`;
}

/** "No sessions in Marketing yet" — a scoped empty list answers "empty for whom". */
export function emptyInProfile(plural: string, profile: string): string {
  return `No ${plural} in ${profile} yet`;
}

/**
 * The aggregate's empty state.
 *
 * It must not name a profile: the aggregate is bounded by no single one, and
 * naming the create target would tell the operator a list is empty in `default`
 * when what is empty is the machine.
 */
export function emptyAcrossProfiles(plural: string): string {
  return `No ${plural} in any profile yet`;
}

/** The sentence a listing's empty state should use for the scope it is showing. */
export function emptyForScope(plural: string, scopeLabel: string | null): string {
  return scopeLabel === null ? emptyAcrossProfiles(plural) : emptyInProfile(plural, scopeLabel);
}

/** The deep-link banner: state the owner, then offer the move. */
export function ownedByProfileLine(noun: string, profile: string): string {
  return `This ${noun} belongs to ${profile}.`;
}

export function switchToProfileLabel(profile: string): string {
  return `Switch to ${profile}`;
}
