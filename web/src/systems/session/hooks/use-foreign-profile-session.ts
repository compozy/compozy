import { useQuery } from "@tanstack/react-query";

import { ownerFromRow, useProfileReadScope, type ProfileOwner } from "@/systems/profiles";

import { sessionAcrossProfilesOptions } from "../lib/query-options";
import type { SessionPayload } from "../types";

/**
 * What the aggregate lookup currently knows.
 *
 * The states are distinct on purpose: "we have not asked" and "we asked and the
 * session is nowhere" are different facts, and collapsing them into `null` is
 * what let the window redirect out from under an in-flight lookup. Only
 * `missing` licenses treating the session as gone.
 */
export type ForeignProfileSessionState =
  | { status: "disabled" }
  | { status: "loading" }
  | { status: "found"; session: SessionPayload; owner: ProfileOwner }
  | { status: "missing" }
  | { status: "error"; error: Error };

/**
 * Resolves a deep link that points at another profile's session.
 *
 * A profile-scoped get answers 404 for a foreign item — correct, and the reason
 * this hook exists at all. Rather than bouncing the operator, it widens once
 * through the labeled aggregate read so the item can be shown read-only under a
 * banner naming its owner. Nothing around it widens: the surrounding listings
 * stay scoped, because this is one explicit by-id read, not a relaxed list.
 *
 * It never runs while the aggregate is already on — there, the scoped read would
 * not have missed in the first place.
 */
export function useForeignProfileSession(
  sessionId: string | null,
  enabled: boolean
): ForeignProfileSessionState {
  const { aggregate } = useProfileReadScope();
  const id = sessionId?.trim() ?? "";
  const active = enabled && !aggregate && id !== "";
  const query = useQuery(sessionAcrossProfilesOptions(id, active));
  if (!active) return { status: "disabled" };
  if (query.data) {
    return { status: "found", session: query.data, owner: ownerFromRow(query.data) };
  }
  if (query.isPending || query.isFetching) return { status: "loading" };
  if (query.isError) {
    // A 404 here is the answer, not a failure: the session is in no profile.
    if (isNotFound(query.error)) return { status: "missing" };
    return { status: "error", error: asError(query.error) };
  }
  return { status: "missing" };
}

function isNotFound(error: unknown): boolean {
  return typeof error === "object" && error !== null && Reflect.get(error, "status") === 404;
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error("Could not resolve the session owner.");
}
