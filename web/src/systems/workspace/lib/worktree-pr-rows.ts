import type { WorktreePrActionRow } from "../components/worktree-pr-action-rows";
import type { WorktreeExitLadder } from "./worktree-exit-ladder";

export type WorktreePrDialogMode = "create" | "existing" | "browser-only";

export interface WorktreePrDialogModel {
  mode: WorktreePrDialogMode;
  rows: WorktreePrActionRow[];
  /** Daemon reason when the PR action exists but cannot run. */
  blockedReason?: string;
}

/**
 * Derives the PR step's footer actions from the exit plan alone.
 *
 * Every row exists because the payload offers the action behind it: with no
 * forge serving the remote there is no create row at all, only the compare link
 * the daemon resolved. Nothing here is disabled-as-placeholder.
 */
export function toWorktreePrDialogModel(ladder: WorktreeExitLadder): WorktreePrDialogModel {
  const openRow = ladder.rows.find(row => row.action === "open_pr" && !row.url);
  const viewRow = ladder.rows.find(row => row.prNumber !== undefined && row.url);
  const browserRow = ladder.rows.find(row => row.url && row.prNumber === undefined);
  const rows: WorktreePrActionRow[] = [];

  if (viewRow) {
    rows.push({ key: "view", label: viewRow.label, primary: true, url: viewRow.url });
    return { mode: "existing", rows };
  }

  if (openRow && ladder.forge) {
    if (ladder.forge.supports_draft) {
      rows.push({
        key: "draft",
        label: `${ladder.forge.open_action_label} (draft)`,
        blocked: !openRow.enabled,
        reason: openRow.blockedReason,
      });
    }
    rows.push({
      key: "create",
      label: openRow.label,
      primary: true,
      blocked: !openRow.enabled,
      reason: openRow.blockedReason,
    });
  }

  const browserURL = browserRow?.url ?? ladder.browserURL;
  if (browserURL) {
    rows.push({
      key: "browser",
      label: browserRow?.label ?? `Open ${ladder.forge?.request_noun ?? "pull request"} in browser`,
      primary: rows.length === 0,
      url: browserURL,
    });
  }

  return {
    mode: rows.some(row => row.key === "create") ? "create" : "browser-only",
    rows,
    blockedReason: openRow?.enabled === false ? openRow.blockedReason : undefined,
  };
}
