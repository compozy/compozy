import type { SessionTimelineToolPart } from "./session-timeline.logic";

/** Local identity retained across streaming re-derivations of one message. */
export interface SessionWorkGroupAnchor {
  groupId: string;
  turnId?: string;
  anchorToolCallId: string;
}

interface WorkGroupIdentityOptions {
  expandedWorkGroupIds?: ReadonlySet<string>;
  workGroupAnchors?: ReadonlyMap<string, SessionWorkGroupAnchor>;
  usedGroupIds?: ReadonlySet<string>;
}

export function workGroupId(
  entries: readonly SessionTimelineToolPart[],
  options: WorkGroupIdentityOptions
): string {
  const first = entries[0]!;
  const turnPrefix = `work:${first.turnId ?? "none"}:`;
  const identities = entries.map(tool => tool.toolCallId.trim() || tool.id);
  const identitySet = new Set(identities);

  const retainedAnchor = [...(options.workGroupAnchors?.values() ?? [])].find(
    anchor =>
      anchor.turnId === first.turnId &&
      identitySet.has(anchor.anchorToolCallId) &&
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
