/** Scripts that wait for the operator across every always-on placement, for the shell's bell. */
import type { AgentInstancePayload } from "../types";

const USER_SCOPE = "@user";

export interface ScriptApprovalItem {
  /** One per approval: profile, resolved script file and bytes. Placements of one file share it. */
  key: string;
  agentName: string;
  agentLabel: string;
  script: string;
  sha256: string;
  /** The selector approval and Heartbeat reads send: a project id or `@user`. */
  workspace: string;
  profile: string;
}

/**
 * The scripts whose current bytes wait for approval, first placement wins, in list order. The
 * daemon reads approval from the script itself, so paused and never-checked placements count.
 */
export function scriptApprovalsFromInstances(
  instances: readonly AgentInstancePayload[]
): ScriptApprovalItem[] {
  const items = new Map<string, ScriptApprovalItem>();
  for (const instance of instances) {
    for (const source of instance.sources ?? []) {
      if (!source.awaiting_approval || !source.script || !source.script_sha256) continue;
      const profile = instance.profile_name;
      // Approvals are keyed by profile and resolved script path; copies in two projects are two.
      const identity =
        source.script_path || [instance.workspace_id, instance.agent_name, source.script].join(":");
      const key = [profile, identity, source.script_sha256].join("\n");
      if (items.has(key)) continue;
      items.set(key, {
        key,
        agentName: instance.agent_name,
        agentLabel: instance.label || instance.agent_name,
        script: source.script,
        sha256: source.script_sha256,
        workspace: instance.workspace_id || USER_SCOPE,
        profile,
      });
    }
  }
  return [...items.values()];
}
