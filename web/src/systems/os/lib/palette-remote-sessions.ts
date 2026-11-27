import type { MergedSessionRow } from "@/systems/computers";
import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";
import {
  sortWorktreeNestEntries,
  toWorktreeNestEntries,
  type DataScope,
  type WorktreeListingByWorkspace,
  type WorktreesResponse,
} from "@/systems/workspace";

import type {
  OsPaletteSessionResult,
  OsPaletteWorktreeResult,
} from "../hooks/use-os-palette-entities";

/**
 * Projects remote computer sessions (including cached offline sessions) into
 * command palette rows carrying their computer label and route search parameter.
 */
export function mergeRemotePaletteSessions(
  localRows: readonly OsPaletteSessionResult[],
  remoteRows: readonly MergedSessionRow<SessionPayload>[]
): OsPaletteSessionResult[] {
  if (remoteRows.length === 0) return [...localRows];
  const seen = new Set(localRows.map(row => row.sessionId));
  const merged: OsPaletteSessionResult[] = [...localRows];
  for (const row of remoteRows) {
    if (seen.has(row.session.id)) continue;
    seen.add(row.session.id);
    const agentName = row.session.agent_name?.trim() || "general";
    const workspaceId = row.session.workspace_id ?? "";
    merged.push({
      sessionId: row.session.id,
      title: getSessionDisplayTitle(row.session),
      agentName,
      workspaceId,
      workspaceLabel: row.offline ? `${row.computerName} · Offline` : row.computerName,
      computerId: row.computerId,
      computerName: row.computerName,
      computerOffline: row.offline,
      route: {
        pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(row.session.id)}`,
        search: { computer: row.computerId },
      },
    });
  }
  return merged;
}

/** The aggregate listing split back into per-project listings (adopted rows only). */
export function groupWorktreeAggregate(
  aggregate: WorktreesResponse | undefined
): WorktreeListingByWorkspace {
  if (!aggregate) return {};
  const listings: Record<string, WorktreesResponse> = {};
  for (const worktree of aggregate.worktrees) {
    const listing = (listings[worktree.workspace_id] ??= {
      worktrees: [],
      discovered: [],
      repo: aggregate.repo,
    });
    listing.worktrees.push(worktree);
  }
  return listings;
}

export function projectPaletteWorktrees(
  dataScope: DataScope | null,
  workspaces: ReadonlyArray<{ id: string; name: string }>,
  worktreeListings: WorktreeListingByWorkspace,
  workspaceWorktrees: WorktreesResponse | undefined
): OsPaletteWorktreeResult[] {
  const worktrees: OsPaletteWorktreeResult[] = [];
  if (dataScope?.kind === "all") {
    for (const workspace of workspaces) {
      const entries = sortWorktreeNestEntries(
        toWorktreeNestEntries(worktreeListings[workspace.id])
      );
      for (const entry of entries) {
        if (entry.kind !== "worktree" || entry.displayState !== "ready") continue;
        worktrees.push({ ...entry, workspaceId: workspace.id, workspaceLabel: workspace.name });
      }
    }
  } else if (dataScope?.kind === "project") {
    worktrees.push(
      ...sortWorktreeNestEntries(toWorktreeNestEntries(workspaceWorktrees)).filter(
        entry => entry.kind === "worktree" && entry.displayState === "ready"
      )
    );
  }
  return worktrees;
}
