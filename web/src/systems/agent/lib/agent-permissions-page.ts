import { permissionRulesEqual, invalidPermissionRule } from "./agent-permission-rules";
import type { AgentSessionApproval, AgentSettingsDraft } from "./agent-settings-draft";

/** How many of the page's three settings differ from the saved definition. */
export function permissionsPageChanges(
  draft: AgentSettingsDraft,
  baseline: AgentSettingsDraft
): number {
  return (
    (draft.permissions === baseline.permissions ? 0 : 1) +
    (draft.sessionApproval === baseline.sessionApproval ? 0 : 1) +
    (permissionRulesEqual(draft.permissionRules, baseline.permissionRules) ? 0 : 1)
  );
}

/** Why Save can't run yet on the Permissions page, in the page's own words. */
export function permissionsPageBlocked(
  draft: AgentSettingsDraft,
  saveBlocked: boolean,
  saveBlockedCaption: string | undefined
): string | undefined {
  if (invalidPermissionRule(draft.permissionRules)) return "Fix the exception first";
  if (!saveBlocked) return undefined;
  return draft.permissions === null ? "Pick a level first" : saveBlockedCaption;
}

/** `children` is what an unset reach already means, so picking it over an unset baseline is no change. */
export function nextSessionApproval(
  approval: AgentSessionApproval,
  baseline: AgentSettingsDraft
): AgentSessionApproval {
  return approval === "children" && baseline.sessionApproval === "" ? "" : approval;
}

/** Whether a running session of the agent asked for the reviewer and fell back. */
export function hasDegradedRunningSession(
  sessions: readonly { state: string; permission_mode_degraded?: boolean }[]
): boolean {
  return sessions.some(
    session =>
      (session.state === "active" || session.state === "starting") &&
      Boolean(session.permission_mode_degraded)
  );
}
