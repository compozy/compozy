import { LOOP_TARGET_KIND, type AutomationLoopTarget } from "./automation-drafts";

interface AutomationTargetSource {
  agent_name: string;
  prompt: string;
  target_kind?: string | null;
  loop_target?: AutomationLoopTarget | null;
}

export interface AgentTargetProjection {
  kind: "agent";
  agentName: string;
  prompt: string;
}

export interface LoopTargetProjection {
  kind: "loop";
  inputMapping: Record<string, string>;
  inputs: Record<string, unknown>;
  loopName: string;
  workspaceId: string;
}

export type AutomationTargetProjection = AgentTargetProjection | LoopTargetProjection;

/**
 * Projects the runtime target union without inferring an agent from its empty contract fields.
 * The discriminator is authoritative: a Loop target stays a Loop even while its draft is incomplete.
 */
export function projectAutomationTarget(
  source: AutomationTargetSource
): AutomationTargetProjection {
  if (source.target_kind === LOOP_TARGET_KIND) {
    return {
      kind: "loop",
      inputMapping: { ...source.loop_target?.input_mapping },
      inputs: { ...source.loop_target?.inputs },
      loopName: source.loop_target?.loop_name ?? "",
      workspaceId: source.loop_target?.workspace_id ?? "",
    };
  }

  return {
    kind: "agent",
    agentName: source.agent_name,
    prompt: source.prompt,
  };
}

function formatInputValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "undefined";
  return JSON.stringify(value) ?? String(value);
}

/** Compact persisted-row summary that keeps typed static input values visible. */
export function describeAutomationTarget(target: AutomationTargetProjection): string {
  if (target.kind === "agent") {
    return target.prompt;
  }

  const inputs = Object.entries(target.inputs)
    .map(([key, value]) => `${key}: ${formatInputValue(value)}`)
    .join(" · ");
  const loop = `Loop ${target.loopName || "not selected"}`;
  return inputs ? `${loop} · ${inputs}` : loop;
}

/** Short target identity for detail metadata. */
export function automationTargetLabel(target: AutomationTargetProjection): string {
  return target.kind === "loop"
    ? `Loop: ${target.loopName || "not selected"}`
    : `Agent: ${target.agentName}`;
}
