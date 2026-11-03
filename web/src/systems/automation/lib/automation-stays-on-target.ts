// S10: what the "Tell a stays-on agent" target can reach. The daemon resolves the instance in the
// trigger's own scope (`internal/automation/dispatch_instance.go`), so picking a scope moves the
// trigger there; a link start always lives in User, so only Global instances fit it.
import type { AgentInstancePayload } from "@/systems/agent";

export interface StaysOnScopeOption {
  /** `""` is Global. */
  workspaceId: string;
  label: string;
  disabledReason?: string;
}

export type StaysOnTargetStatus =
  | { kind: "unset" }
  | { kind: "missing"; agent: string; scope: string }
  | { kind: "paused"; label: string }
  | { kind: "no-checks"; label: string }
  | { kind: "ready"; label: string };

export interface StaysOnAgentOption {
  /** The agent name the trigger saves. */
  name: string;
  /** What the operator reads: the instance's display name. */
  label: string;
}

export interface StaysOnTargetModel {
  agents: StaysOnAgentOption[];
  scopes: StaysOnScopeOption[];
  /** `""` is Global, `null` when nothing is chosen yet. */
  selectedScope: string | null;
  status: StaysOnTargetStatus;
}

const LINK_GLOBAL_ONLY = "Link automations live in User, so they reach Global agents only";

function scopeLabel(instance: Pick<AgentInstancePayload, "workspace_id" | "scope">) {
  return instance.workspace_id ? instance.scope || instance.workspace_id : "Global";
}

export function staysOnTargetModel(input: {
  /** Instances in the trigger's own profile: the daemon resolves the target only there. */
  instances: readonly AgentInstancePayload[];
  agent: string;
  /** The trigger's own scope: `""` for User. */
  workspaceId: string;
  isWebhook: boolean;
}): StaysOnTargetModel {
  const agent = input.agent.trim();
  const labels = new Map<string, string>();
  for (const instance of input.instances) {
    if (!labels.has(instance.agent_name)) {
      labels.set(instance.agent_name, instance.label || instance.agent_name);
    }
  }
  const agents = [...labels]
    .map(([name, label]) => ({ name, label }))
    .sort((a, b) => a.label.localeCompare(b.label));
  const own = input.instances.filter(instance => instance.agent_name === agent);
  const scopes = own
    .map(instance => ({
      workspaceId: instance.workspace_id,
      label: scopeLabel(instance),
      disabledReason: input.isWebhook && instance.workspace_id ? LINK_GLOBAL_ONLY : undefined,
    }))
    .sort((a, b) => (a.workspaceId === "" ? -1 : b.workspaceId === "" ? 1 : 0));
  const target = input.isWebhook ? "" : input.workspaceId;
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
  const status: StaysOnTargetStatus =
    instance.state === "paused"
      ? { kind: "paused", label }
      : instance.checks_enabled
        ? { kind: "ready", label }
        : { kind: "no-checks", label };
  return { agents, scopes, selectedScope: instance.workspace_id, status };
}
