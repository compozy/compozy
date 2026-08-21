import { PROFILE_AGGREGATE, type ProfileView } from "../types";

/**
 * The two read modes the daemon accepts — one profile, or the explicit labeled
 * aggregate. There is no third mode: omitting both would mean "unscoped", which
 * the store refuses (ADR-005, ADR-015).
 */
export type ProfileScopeParams = { profile: string; all_profiles?: never } | { all_profiles: true };
export type ProfileMutationScopeParams = { profile: string };

/** Every profile-owned row the daemon labels, whatever surface it came from. */
export interface ProfileOwnerLabel {
  profile_id: string;
  profile_name: string;
  profile_color?: string;
  profile_icon?: string;
  profile_emoji?: string;
  profile_archived?: boolean;
}

/** The identity an owner tag, banner, or breakdown row renders. */
export interface ProfileOwner {
  id: string;
  name: string;
  color?: string;
  icon?: string | null;
  emoji?: string | null;
  archived: boolean;
}

/**
 * What a listing needs to know about the profile axis: whether to tag rows, who
 * to name when the list is empty, and how to resolve a row's owner. Presentational
 * listings take this rather than reading the shell themselves.
 */
export interface ProfileListingScope {
  aggregate: boolean;
  /**
   * The profile a *new* item would land in. Under the aggregate that is
   * `default`, which is why it must never be read as the name of what is on
   * screen: an empty aggregate list is not "empty in default".
   */
  destination: string;
  /**
   * The profile the list is *showing*, or `null` under the aggregate where no
   * single profile bounds it. This is the one an empty state may name.
   */
  scopeLabel: string | null;
  ownerOf: (row: ProfileOwnerLabel) => ProfileOwner;
}

export function isAggregateView(view: ProfileView): boolean {
  return view.kind === "aggregate";
}

/** Query params for one work read. Aggregate widening is always explicit. */
export function profileScopeParams(view: ProfileView): ProfileScopeParams {
  return view.kind === "aggregate" ? { all_profiles: true } : { profile: view.profile };
}

/**
 * The cache-key segment for a view.
 *
 * The aggregate uses the daemon's reserved `@all` identity rather than a profile
 * name, so aggregate results can never collide with — or be mistaken for — the
 * `default` profile's own cache entry (ADR-016).
 */
export function profileViewKey(view: ProfileView): string {
  return view.kind === "aggregate" ? PROFILE_AGGREGATE : view.profile;
}

/** Reads the owner off any labeled row without knowing which surface produced it. */
export function ownerFromRow(row: ProfileOwnerLabel): ProfileOwner {
  return {
    id: row.profile_id,
    name: row.profile_name,
    color: row.profile_color,
    icon: row.profile_icon ?? null,
    emoji: row.profile_emoji ?? null,
    archived: row.profile_archived === true,
  };
}
