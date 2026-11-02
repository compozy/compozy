import { formatRelativeClock } from "./agent-time";
import type { AgentTeamPlace } from "./agent-team";
import type { AgentInstanceReport } from "../types";

export interface AgentFinding {
  id: string;
  title: string;
  scope: string;
  when: string;
  /** Waiting for a decision from you. */
  waiting: boolean;
  sessionId?: string | null;
  workspaceId: string;
}

const FINDINGS_SHOWN = 5;

const FAILURE_TITLE: Record<string, string> = {
  failure: "A check-in failed",
  approval_expired: "An approval expired",
  interrupted: "A run was interrupted",
  rotation_failed: "It couldn't start a fresh conversation",
  rotation_deferred: "A fresh conversation is waiting",
};

/** Newest proposals first, read from each place's reports. */
export function agentFindings(
  reports: readonly { place: AgentTeamPlace; report: AgentInstanceReport }[],
  now = Date.now()
): AgentFinding[] {
  return [...reports]
    .sort((left, right) => right.report.created_at.localeCompare(left.report.created_at))
    .flatMap(({ place, report }) => {
      const titles =
        report.kind === "proposal"
          ? (report.proposals ?? []).map(proposal => proposal.title)
          : [FAILURE_TITLE[report.kind] ?? "It told you something"];
      return titles.map((title, index) => ({
        id: `${report.id}-${index}`,
        title,
        scope: place.label,
        when: formatRelativeClock(report.created_at, now) ?? "",
        waiting: report.state === "pending",
        sessionId: place.instance.main_session_id,
        workspaceId: place.instance.workspace_id,
      }));
    })
    .slice(0, FINDINGS_SHOWN);
}
