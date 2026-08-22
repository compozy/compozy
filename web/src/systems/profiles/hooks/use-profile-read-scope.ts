import { actingProfile, activeProfiles, isArchived } from "../lib/profile-rows";
import {
  ownerFromRow,
  profileScopeParams,
  profileViewKey,
  type ProfileOwner,
  type ProfileListingScope,
  type ProfileOwnerLabel,
  type ProfileScopeParams,
} from "../lib/profile-scope";
import type { ProfileLens, ProfilePayload, ProfileView } from "../types";
import { useProfileLens } from "./use-profile-lens";
import { useActiveProfileView } from "./use-profile-selection";
import { useProfiles } from "./use-profiles";

export interface ProfileReadScope extends ProfileListingScope {
  lens: ProfileLens;
  view: ProfileView;
  /** True while the labeled aggregate is on. Owner tags appear only here. */
  aggregate: boolean;
  /** Cache-key segment: a profile name, or the reserved `@all`. */
  key: string;
  /** The scope every work read sends. Never omitted, never client-side. */
  params: ProfileScopeParams;
  /** The profile anything created right now belongs to (ADR-005). */
  destination: string;
  /** The profile the current reads are bounded by, or `null` under the aggregate. */
  scopeLabel: string | null;
  /** Full identity of the destination, once the profile list has resolved. */
  destinationOwner: ProfileOwner | null;
  /** A second profile exists — nothing announces profiles before that. */
  plural: boolean;
  /**
   * Completes a row's own labels with the state only the profile list knows.
   * Row labels stay authoritative for name and identity; the list adds whether
   * the owner has since been archived, which work rows do not carry.
   */
  ownerOf: (row: ProfileOwnerLabel) => ProfileOwner;
}

/**
 * The single seam between the shell's active view and every scoped read.
 *
 * Listings, streams, palette caches, creation surfaces, and usage breakdowns all
 * read this rather than deriving scope themselves, so one switch moves them
 * together and no surface can silently fall back to an unscoped read.
 */
export function useProfileReadScope(): ProfileReadScope {
  const lens = useProfileLens();
  const view = useActiveProfileView(lens);
  const profiles = useProfiles();
  const rows = profiles.data ?? [];
  const archivedById = archivedLookup(rows);
  return {
    lens,
    view,
    aggregate: view.kind === "aggregate",
    key: profileViewKey(view),
    params: profileScopeParams(view),
    destination: actingProfile(view),
    scopeLabel: view.kind === "profile" ? view.profile : null,
    destinationOwner: ownerNamed(rows, actingProfile(view)),
    plural: activeProfiles(rows).length > 1,
    ownerOf: row => {
      const owner = ownerFromRow(row);
      if (row.profile_archived !== undefined) return owner;
      return { ...owner, archived: archivedById.get(owner.id) ?? false };
    },
  };
}

function archivedLookup(rows: readonly ProfilePayload[]): Map<string, boolean> {
  return new Map(rows.map(row => [row.id, isArchived(row)]));
}

function ownerNamed(rows: readonly ProfilePayload[], name: string): ProfileOwner | null {
  const match = rows.find(row => row.name === name);
  if (!match) return null;
  return {
    id: match.id,
    name: match.name,
    color: match.color,
    icon: match.icon,
    emoji: match.emoji,
    archived: isArchived(match),
  };
}

/**
 * The destination chip's value: the profile a creation lands in while the
 * aggregate is on, and `null` otherwise.
 *
 * Creation surfaces read this at the connected edge and pass the result down, so
 * the presentational statement stays a pure function of its props.
 */
export function useAggregateDestination(): string | null {
  const { aggregate, destination } = useProfileReadScope();
  return aggregate ? destination : null;
}
