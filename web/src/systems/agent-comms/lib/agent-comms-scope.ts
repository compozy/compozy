/**
 * The read scope every calls query carries.
 *
 * Workspace and profile travel together because both bound the same reads: the
 * workspace pins the population, the profile pins ownership. Bundling them into
 * one value keeps query-option signatures readable and — more importantly —
 * makes it impossible to pass the cache-key segment without the matching request
 * params, which is how a surface would silently read one scope's rows under
 * another scope's key.
 */
import type { ProfileScopeParams } from "@/systems/profiles";

export interface AgentCommsScope {
  /** Empty while the active workspace is still resolving. */
  workspaceId: string;
  /** Cache-key segment: a profile name, or the reserved `@all`. */
  profileKey: string;
  /** The scope params every request sends. Never omitted, never client-side. */
  params: ProfileScopeParams;
  /** The profile a mutation acts as. Aggregate reads still write to one profile. */
  actingProfile: string;
}

/** A scope can serve a read only once the workspace has resolved. */
export function isScopeReady(scope: AgentCommsScope): boolean {
  return scope.workspaceId.length > 0;
}
