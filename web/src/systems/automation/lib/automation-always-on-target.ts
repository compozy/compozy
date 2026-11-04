// S10: what the "Tell an always-on agent" target can reach. The daemon resolves the instance in the
// trigger's own scope (`internal/automation/dispatch_instance.go`), so picking a scope moves the
// trigger there. A link that tells an always-on agent keeps that scope too, since the daemon serves
// project links under `/api/webhooks/workspaces/{workspace_id}/…`.
import type { AgentInstancePayload } from "@/systems/agent";

export interface AlwaysOnScopeOption {
  /** `""` is Global. */
  workspaceId: string;
  label: string;
}

export type AlwaysOnTargetStatus =
  | { kind: "unset" }
  | { kind: "missing"; agent: string; scope: string }
  | { kind: "paused"; label: string }
  | { kind: "no-checks"; label: string }
  | { kind: "ready"; label: string };

export interface AlwaysOnAgentOption {
  /** The agent name the trigger saves. */
  name: string;
  /** What the operator reads: the instance's display name. */
  label: string;
}

export interface AlwaysOnTargetModel {
  agents: AlwaysOnAgentOption[];
  scopes: AlwaysOnScopeOption[];
  /** `""` is Global, `null` when nothing is chosen yet. */
  selectedScope: string | null;
  status: AlwaysOnTargetStatus;
}

function scopeLabel(instance: Pick<AgentInstancePayload, "workspace_id" | "scope">) {
  return instance.workspace_id ? instance.scope || instance.workspace_id : "Global";
}

export function alwaysOnTargetModel(input: {
  /** Instances in the trigger's own profile: the daemon resolves the target only there. */
  instances: readonly AgentInstancePayload[];
  agent: string;
  /** The trigger's own scope: `""` for User. */
  workspaceId: string;
}): AlwaysOnTargetModel {
  const agent = input.agent.trim();
  const labels = new Map<string, string>();
  for (const instance of input.instances) {
    if (!labels.has(instance.agent_name)) {
      labels.set(instance.agent_name, instance.label || instance.agent_name);
    }
  }
  const counts = new Map<string, number>();
  for (const label of labels.values()) counts.set(label, (counts.get(label) ?? 0) + 1);
  // Two agents can share a display name; only then does the name tell them apart.
  const agents = [...labels]
    .sort(
      ([nameA, labelA], [nameB, labelB]) =>
        labelA.localeCompare(labelB) || nameA.localeCompare(nameB)
    )
    .map(([name, label]) => ({
      name,
      label: (counts.get(label) ?? 0) > 1 ? `${label} (${name})` : label,
    }));
  const own = input.instances.filter(instance => instance.agent_name === agent);
  const scopes = own
    .map(instance => ({
      workspaceId: instance.workspace_id,
      label: scopeLabel(instance),
    }))
    .sort((a, b) => (a.workspaceId === "" ? -1 : b.workspaceId === "" ? 1 : 0));
  const target = input.workspaceId;
  const instance = own.find(candidate => candidate.workspace_id === target);
  if (!agent) return { agents, scopes, selectedScope: null, status: { kind: "unset" } };
  if (!instance) {
    return {
      agents,
      scopes,
      selectedScope: null,
      status: { kind: "missing", agent, scope: target ? "this project" : "Global" },
    };
  }
  const label = instance.label || agent;
  const status: AlwaysOnTargetStatus =
    instance.state === "paused"
      ? { kind: "paused", label }
      : instance.checks_enabled
        ? { kind: "ready", label }
        : { kind: "no-checks", label };
  return { agents, scopes, selectedScope: instance.workspace_id, status };
}
