import type { SessionWorkEntry } from "./session-timeline.logic";

/** Local identity retained across streaming re-derivations of one message. */
export interface SessionWorkGroupAnchor {
  groupId: string;
  turnId?: string;
  anchorEntryId: string;
}

interface WorkGroupIdentityOptions {
  expandedWorkGroupIds?: ReadonlySet<string>;
  workGroupAnchors?: ReadonlyMap<string, SessionWorkGroupAnchor>;
  usedGroupIds?: ReadonlySet<string>;
}

/** Retain an existing entry anchor across streaming updates without reusing a sibling group id. */
export function workGroupId(
  entries: readonly SessionWorkEntry[],
  options: WorkGroupIdentityOptions
): string {
  const first = entries[0]!;
  const turnPrefix = `work:${first.turnId ?? "none"}:`;
  const identities = entries.map(workEntryIdentity);
  const identitySet = new Set(identities);

  const retainedAnchor = [...(options.workGroupAnchors?.values() ?? [])].find(
    anchor =>
      anchor.turnId === first.turnId &&
      identitySet.has(anchor.anchorEntryId) &&
      !options.usedGroupIds?.has(anchor.groupId)
  );
  if (retainedAnchor) return retainedAnchor.groupId;

  const orderedIdentities = [...new Set(identities)].sort(compareIdentities);
  const identity = orderedIdentities[0] ?? first.id;

  // A deterministic anchor keeps a reorder from changing the row key. If a
  // new call sorts before the old anchor, retain the expanded group's existing
  // id when its logical tool call is still present in this run.
  const expandedGroupId = [...(options.expandedWorkGroupIds ?? [])].find(groupId => {
    if (!groupId.startsWith(turnPrefix)) return false;
    const expandedIdentity = groupId.slice(turnPrefix.length);
    return identitySet.has(expandedIdentity);
  });
  const candidate = expandedGroupId ?? `${turnPrefix}${identity}`;
  const usedGroupIds = options.usedGroupIds;
  if (!usedGroupIds || !usedGroupIds.has(candidate)) return candidate;

  let suffix = 2;
  let uniqueCandidate = `${candidate}:part-${suffix}`;
  while (usedGroupIds.has(uniqueCandidate)) {
    suffix += 1;
    uniqueCandidate = `${candidate}:part-${suffix}`;
  }
  return uniqueCandidate;
}

function compareIdentities(left: string, right: string): number {
  if (left === right) return 0;
  return left < right ? -1 : 1;
}

/** Use the logical tool call or a namespaced reasoning part as the group anchor. */
export function workEntryIdentity(entry: SessionWorkEntry): string {
  return entry.kind === "tool" ? entry.toolCallId.trim() || entry.id : `reasoning:${entry.id}`;
}
