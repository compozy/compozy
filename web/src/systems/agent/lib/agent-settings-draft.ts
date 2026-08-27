import { isReasoningEffort, type ReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";

import {
  AGENT_CREATE_PERMISSION_OPTIONS,
  parseAgentCreateCategoryPath,
  type AgentCreatePermission,
  type AgentCreatePermissionChoice,
} from "./agent-create-draft";
import { joinAgentCategorySegments } from "./agent-category";
import { runtimeACPSelections } from "./agent-effective-runtime";
import type { AgentPayload, UpdateAgentParams } from "../types";
import type { RuntimeACPOptionSelection } from "@/systems/runtime";

const KNOWN_PERMISSIONS = new Set<AgentCreatePermission>(
  AGENT_CREATE_PERMISSION_OPTIONS.map(option => option.value).filter(
    (value): value is AgentCreatePermission => value !== ""
  )
);

export interface AgentSettingsDraft {
  name: string;
  categoryPath: string;
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort | "";
  speed: RuntimeSpeed | "";
  /** Typed ACP overrides; undefined means inherit provider defaults. */
  acpOptions?: RuntimeACPOptionSelection[];
  command: string;
  prompt: string;
  /** Selected RadioCard value; null when legacy string requires explicit choice. */
  permissions: AgentCreatePermissionChoice | null;
  /** Raw permissions from the read payload when unrecognized. */
  legacyPermissions: string | null;
  tools: string[];
  toolsets: string[];
  denyTools: string[];
  disabledSkills: string[];
  definitionDigest: string;
  origin: AgentPayload["origin"];
}

export interface AgentSettingsValidation {
  fields: Partial<Record<"provider" | "prompt" | "categoryPath" | "permissions", string>>;
  canSave: boolean;
  categorySegments: string[];
  requiresExplicitPermissions: boolean;
}

export function isKnownPermission(
  value: string | null | undefined
): value is AgentCreatePermission {
  return Boolean(value && KNOWN_PERMISSIONS.has(value as AgentCreatePermission));
}

export function buildSettingsDraftFromAgent(agent: AgentPayload): AgentSettingsDraft {
  const rawPermissions = agent.permissions?.trim() ?? "";
  const known = isKnownPermission(rawPermissions);
  const emptyOrInherit = rawPermissions.length === 0;
  return {
    name: agent.name,
    categoryPath: joinAgentCategorySegments(agent.category_path ?? []),
    provider: agent.provider ?? "",
    model: agent.model ?? "",
    reasoningEffort:
      agent.reasoning_effort && isReasoningEffort(agent.reasoning_effort)
        ? agent.reasoning_effort
        : "",
    speed: agentRuntimeSpeed(agent.speed),
    acpOptions: runtimeACPSelections(agent.acp_options),
    command: agent.command ?? "",
    prompt: agent.prompt ?? "",
    permissions: known ? rawPermissions : emptyOrInherit ? "" : null,
    legacyPermissions: known || emptyOrInherit ? null : rawPermissions,
    tools: [...(agent.tools ?? [])],
    toolsets: [...(agent.toolsets ?? [])],
    denyTools: [...(agent.deny_tools ?? [])],
    disabledSkills: [...(agent.skills?.disabled ?? [])],
    definitionDigest: agent.definition_digest,
    origin: agent.origin,
  };
}

export function isAgentSettingsDraftDirty(draft: AgentSettingsDraft, agent: AgentPayload): boolean {
  const baseline = buildSettingsDraftFromAgent(agent);
  return (
    draft.categoryPath !== baseline.categoryPath ||
    draft.provider !== baseline.provider ||
    draft.model !== baseline.model ||
    draft.reasoningEffort !== baseline.reasoningEffort ||
    draft.speed !== baseline.speed ||
    !sameOptionalList(draft.acpOptions, baseline.acpOptions) ||
    draft.command !== baseline.command ||
    draft.prompt !== baseline.prompt ||
    draft.permissions !== baseline.permissions ||
    !sameList(draft.tools, baseline.tools) ||
    !sameList(draft.toolsets, baseline.toolsets) ||
    !sameList(draft.denyTools, baseline.denyTools) ||
    !sameList(draft.disabledSkills, baseline.disabledSkills)
  );
}

export function validateAgentSettingsDraft(draft: AgentSettingsDraft): AgentSettingsValidation {
  const fields: AgentSettingsValidation["fields"] = {};
  const category = parseAgentCreateCategoryPath(draft.categoryPath);
  if (category.error) fields.categoryPath = category.error;

  if (
    draft.provider.trim().length === 0 &&
    (draft.model.trim().length > 0 || draft.reasoningEffort !== "")
  ) {
    fields.provider = "Choose a provider before setting model or reasoning overrides.";
  }
  if (draft.prompt.trim().length === 0) {
    fields.prompt = "Prompt is required.";
  }

  const requiresExplicitPermissions = draft.permissions === null;
  if (requiresExplicitPermissions) {
    fields.permissions = draft.legacyPermissions
      ? `Unrecognized permission mode: ${draft.legacyPermissions}`
      : "Choose a permission mode.";
  }

  return {
    fields,
    canSave: Object.keys(fields).length === 0,
    categorySegments: category.segments,
    requiresExplicitPermissions,
  };
}

export function buildUpdateAgentParams(
  draft: AgentSettingsDraft,
  workspaceId: string | null | undefined
): UpdateAgentParams | null {
  const validation = validateAgentSettingsDraft(draft);
  if (!validation.canSave || draft.permissions === null) return null;

  const provider = draft.provider.trim();
  const prompt = draft.prompt.trim();
  const model = draft.model.trim();
  const command = draft.command.trim();
  const reasoningEffort = isReasoningEffort(draft.reasoningEffort) ? draft.reasoningEffort : "";
  const permissions = draft.permissions === "" ? undefined : draft.permissions;
  const workspace = workspaceId?.trim() ?? "";

  return {
    expected_digest: draft.definitionDigest,
    ...(draft.origin === "workspace" && workspace.length > 0 ? { workspace } : {}),
    agent: {
      name: draft.name,
      prompt,
      ...(provider.length > 0 ? { provider } : {}),
      ...(model.length > 0 ? { model } : {}),
      ...(command.length > 0 ? { command } : {}),
      ...(reasoningEffort !== "" ? { reasoning_effort: reasoningEffort } : {}),
      ...(draft.speed !== "" ? { speed: draft.speed } : {}),
      ...(draft.acpOptions !== undefined ? { acp_options: [...draft.acpOptions] } : {}),
      ...(permissions ? { permissions } : {}),
      ...(validation.categorySegments.length > 0
        ? { category_path: validation.categorySegments }
        : {}),
      ...(draft.tools.length > 0 ? { tools: [...draft.tools] } : {}),
      ...(draft.toolsets.length > 0 ? { toolsets: [...draft.toolsets] } : {}),
      ...(draft.denyTools.length > 0 ? { deny_tools: [...draft.denyTools] } : {}),
      ...(draft.disabledSkills.length > 0
        ? { skills: { disabled: [...draft.disabledSkills] } }
        : { skills: null }),
    },
  };
}

function agentRuntimeSpeed(value: string | null | undefined): RuntimeSpeed | "" {
  return value === "normal" || value === "fast" ? value : "";
}

function sameList(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((value, index) => value === right[index]);
}

function sameOptionalList(
  left: readonly RuntimeACPOptionSelection[] | undefined,
  right: readonly RuntimeACPOptionSelection[] | undefined
): boolean {
  if (left === undefined || right === undefined) return left === right;
  if (left.length !== right.length) return false;
  return left.every(
    (selection, index) =>
      selection.id === right[index]?.id &&
      selection.value_id === right[index]?.value_id &&
      selection.bool_value === right[index]?.bool_value
  );
}
