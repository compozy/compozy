import { isReasoningEffort, type ReasoningEffort } from "@/lib/api-contract";
import type { RuntimeProviderOption } from "@/systems/runtime";

import { joinAgentCategorySegments } from "./agent-category";
import type { AgentPayload, CreateAgentParams, DuplicateAgentParams } from "../types";

export type AgentCreateScope = CreateAgentParams["scope"];
export type AgentCreatePermission = NonNullable<CreateAgentParams["agent"]["permissions"]>;
export type AgentCreatePermissionChoice = "" | AgentCreatePermission;

export interface AgentCreateDialogDraft {
  scope: AgentCreateScope;
  name: string;
  categoryPath: string;
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort | "";
  command: string;
  prompt: string;
  permissions: AgentCreatePermissionChoice;
  tools: string[];
  toolsets: string[];
  denyTools: string[];
  disabledSkills: string[];
}

export interface AgentCreateValidationContext {
  hasActiveWorkspace: boolean;
  providerOptions: readonly RuntimeProviderOption[];
  providersError: string | null;
  providersLoading: boolean;
}

export interface AgentCreateValidation {
  fields: Partial<Record<AgentCreateFieldKey, string>>;
  /**
   * True when every field that only renders under Advanced is valid. Simple can
   * never surface these errors, so a blocked submit has to reveal Advanced
   * instead of leaving the operator with a dead primary.
   */
  advancedValid: boolean;
  /** True when every field Simple renders is valid. */
  simpleValid: boolean;
  canSubmit: boolean;
  categorySegments: string[];
}

/** Advanced-only fields that can produce validation errors hidden by the Simple tier. */
export const AGENT_CREATE_ADVANCED_FIELDS: readonly AgentCreateFieldKey[] = [
  "tools",
  "toolsets",
  "denyTools",
] as const;

export type AgentCreateFieldKey =
  | "name"
  | "scope"
  | "categoryPath"
  | "provider"
  | "reasoningEffort"
  | "prompt"
  | "tools"
  | "toolsets"
  | "denyTools";

export const AGENT_CREATE_PERMISSION_OPTIONS: readonly {
  value: AgentCreatePermissionChoice;
  label: string;
}[] = [
  { value: "", label: "Inherit default" },
  { value: "deny-all", label: "Deny all" },
  { value: "approve-reads", label: "Approve reads" },
  { value: "approve-all", label: "Approve all" },
] as const;

export function createDefaultAgentCreateDraft(hasActiveWorkspace: boolean): AgentCreateDialogDraft {
  return {
    scope: hasActiveWorkspace ? "workspace" : "global",
    name: "",
    categoryPath: "",
    provider: "",
    model: "",
    reasoningEffort: "",
    command: "",
    prompt: "",
    permissions: "",
    tools: [],
    toolsets: [],
    denyTools: [],
    disabledSkills: [],
  };
}

export function updateAgentCreateScope(
  draft: AgentCreateDialogDraft,
  scope: AgentCreateScope
): AgentCreateDialogDraft {
  if (draft.scope === scope) return draft;
  return {
    ...draft,
    scope,
    provider: "",
    model: "",
    reasoningEffort: "",
  };
}

export function appendAgentCreateTokens(current: readonly string[], rawInput: string): string[] {
  const next = [...current];
  const seen = new Set(next);
  for (const token of splitAgentCreateTokens(rawInput)) {
    if (seen.has(token)) continue;
    seen.add(token);
    next.push(token);
  }
  return next;
}

export function removeAgentCreateToken(current: readonly string[], target: string): string[] {
  return current.filter(value => value !== target);
}

export function splitAgentCreateTokens(rawInput: string): string[] {
  const values = rawInput.split(/[,\n]/).flatMap(value => {
    const token = value.trim();
    return token ? [token] : [];
  });
  return [...new Set(values)];
}

export function parseAgentCreateCategoryPath(rawInput: string): {
  segments: string[];
  error: string | null;
} {
  const trimmed = rawInput.trim();
  if (trimmed.length === 0) {
    return { segments: [], error: null };
  }
  if (trimmed.includes("\\")) {
    return { segments: [], error: "Category path cannot contain backslashes." };
  }

  const rawSegments = trimmed.split("/");
  const segments: string[] = [];
  for (const rawSegment of rawSegments) {
    const segment = rawSegment.trim();
    if (segment.length === 0) {
      return { segments: [], error: "Category path cannot contain blank segments." };
    }
    if (segment === "." || segment === "..") {
      return { segments: [], error: "Category path cannot contain . or .. segments." };
    }
    if (segment.includes("\\") || segment.includes("/")) {
      return { segments: [], error: "Category path segments cannot contain path separators." };
    }
    segments.push(segment);
  }

  return { segments, error: null };
}

export function validateAgentCreateDraft(
  draft: AgentCreateDialogDraft,
  context: AgentCreateValidationContext
): AgentCreateValidation {
  const fields: AgentCreateValidation["fields"] = {};
  const name = draft.name.trim();
  if (name.length === 0) {
    fields.name = "Enter an agent name.";
  } else if (name === "." || name === ".." || name.includes("/") || name.includes("\\")) {
    fields.name = "Agent names cannot be . or .. and cannot contain path separators.";
  }

  if (draft.scope === "workspace" && !context.hasActiveWorkspace) {
    fields.scope = "Select an active workspace or switch scope to global.";
  }

  const category = parseAgentCreateCategoryPath(draft.categoryPath);
  if (category.error) {
    fields.categoryPath = category.error;
  }

  const provider = draft.provider.trim();
  const providerKnown = context.providerOptions.some(option => option.id === provider);
  const hasDependentOverride = draft.model.trim().length > 0 || draft.reasoningEffort !== "";
  if (context.providersLoading) {
    fields.provider = "Provider options are still loading.";
  } else if (context.providersError) {
    fields.provider = context.providersError;
  } else if (context.providerOptions.length === 0) {
    fields.provider = "No providers are configured for this scope.";
  } else if (provider.length === 0) {
    if (hasDependentOverride) {
      fields.provider = "Choose a provider before setting model or reasoning overrides.";
    }
  } else if (!providerKnown) {
    fields.provider = "Choose a provider from this scope.";
  }

  // Fail closed on an off-contract effort: a corrupted/foreign draft carrying a
  // value like "ultra" must be rejected here, never silently stripped at the wire
  // boundary. Empty is the canonical "provider default" and stays valid.
  if (draft.reasoningEffort !== "" && !isReasoningEffort(draft.reasoningEffort)) {
    fields.reasoningEffort = "Choose a valid reasoning effort.";
  }

  if (draft.prompt.trim().length === 0) {
    fields.prompt = "Enter the agent instructions.";
  }

  const toolsError = validateToolPatternList(draft.tools, "Tool");
  if (toolsError) fields.tools = toolsError;
  const denyToolsError = validateToolPatternList(draft.denyTools, "Denied tool");
  if (denyToolsError) fields.denyTools = denyToolsError;
  const toolsetsError = validateToolsetList(draft.toolsets);
  if (toolsetsError) fields.toolsets = toolsetsError;

  const advancedValid = AGENT_CREATE_ADVANCED_FIELDS.every(field => !fields[field]);
  const simpleValid = (Object.keys(fields) as AgentCreateFieldKey[]).every(
    field => AGENT_CREATE_ADVANCED_FIELDS.includes(field) || !fields[field]
  );

  return {
    fields,
    advancedValid,
    simpleValid,
    canSubmit: Object.values(fields).every(message => !message),
    categorySegments: category.segments,
  };
}

export function buildCreateAgentParams(
  draft: AgentCreateDialogDraft,
  workspaceId: string | null | undefined,
  context: AgentCreateValidationContext
): CreateAgentParams | null {
  const validation = validateAgentCreateDraft(draft, context);
  if (!validation.canSubmit) return null;
  const normalizedWorkspaceId = workspaceId?.trim() ?? "";
  if (draft.scope === "workspace" && normalizedWorkspaceId.length === 0) {
    return null;
  }

  const name = draft.name.trim();
  const provider = draft.provider.trim();
  const prompt = draft.prompt.trim();
  const model = draft.model.trim();
  // `validateAgentCreateDraft` already fails closed on any off-contract effort, so
  // a corrupted draft never reaches this line (canSubmit is false → null above).
  // The guard here only normalizes "" (provider default) to the omitted form below.
  const reasoningEffort = isReasoningEffort(draft.reasoningEffort) ? draft.reasoningEffort : "";
  const command = draft.command.trim();
  const tools = normalizeOrderedTokens(draft.tools);
  const toolsets = normalizeOrderedTokens(draft.toolsets);
  const denyTools = normalizeOrderedTokens(draft.denyTools);
  const disabledSkills = normalizeOrderedTokens(draft.disabledSkills);
  const permissions = draft.permissions === "" ? undefined : draft.permissions;

  return {
    scope: draft.scope,
    ...(draft.scope === "workspace" ? { workspace: normalizedWorkspaceId } : {}),
    agent: {
      name,
      prompt,
      ...(provider.length > 0 ? { provider } : {}),
      ...(command.length > 0 ? { command } : {}),
      ...(model.length > 0 ? { model } : {}),
      ...(reasoningEffort !== "" ? { reasoning_effort: reasoningEffort } : {}),
      ...(tools.length > 0 ? { tools } : {}),
      ...(toolsets.length > 0 ? { toolsets } : {}),
      ...(denyTools.length > 0 ? { deny_tools: denyTools } : {}),
      ...(permissions ? { permissions } : {}),
      ...(validation.categorySegments.length > 0
        ? { category_path: validation.categorySegments }
        : {}),
      ...(disabledSkills.length > 0 ? { skills: { disabled: disabledSkills } } : {}),
    },
  };
}

const CREATE_PERMISSIONS = new Set<AgentCreatePermission>([
  "deny-all",
  "approve-reads",
  "approve-all",
]);

function mapPermissionsChoice(permissions: string | undefined): AgentCreatePermissionChoice {
  if (!permissions) return "";
  return CREATE_PERMISSIONS.has(permissions as AgentCreatePermission)
    ? (permissions as AgentCreatePermission)
    : "";
}

/** Prefill the create wizard from a read payload; name stays empty for the operator. */
export function buildDraftFromAgentPayload(
  agent: AgentPayload,
  hasActiveWorkspace: boolean
): AgentCreateDialogDraft {
  const scope: AgentCreateScope =
    agent.origin === "workspace" && hasActiveWorkspace ? "workspace" : "global";
  const reasoning =
    agent.reasoning_effort && isReasoningEffort(agent.reasoning_effort)
      ? agent.reasoning_effort
      : "";
  return {
    scope,
    name: "",
    categoryPath: joinAgentCategorySegments(agent.category_path ?? []),
    provider: agent.provider ?? "",
    model: agent.model ?? "",
    reasoningEffort: reasoning,
    command: agent.command ?? "",
    prompt: agent.prompt ?? "",
    permissions: mapPermissionsChoice(agent.permissions),
    tools: [...(agent.tools ?? [])],
    toolsets: [...(agent.toolsets ?? [])],
    denyTools: [...(agent.deny_tools ?? [])],
    disabledSkills: [...(agent.skills?.disabled ?? [])],
  };
}

function sameTokenList(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((value, index) => value === right[index]);
}

/**
 * Build POST /duplicate body: required target name + override diff against source.
 * Never returns a create payload.
 */
export function buildDuplicateAgentParams(
  source: AgentPayload,
  draft: AgentCreateDialogDraft,
  workspaceId: string | null | undefined,
  context: AgentCreateValidationContext
): DuplicateAgentParams | null {
  const validation = validateAgentCreateDraft(draft, context);
  if (!validation.canSubmit) return null;
  const normalizedWorkspaceId = workspaceId?.trim() ?? "";
  if (draft.scope === "workspace" && normalizedWorkspaceId.length === 0) {
    return null;
  }

  const name = draft.name.trim();
  const provider = draft.provider.trim();
  const prompt = draft.prompt.trim();
  const model = draft.model.trim();
  const reasoningEffort = isReasoningEffort(draft.reasoningEffort) ? draft.reasoningEffort : "";
  const command = draft.command.trim();
  const tools = normalizeOrderedTokens(draft.tools);
  const toolsets = normalizeOrderedTokens(draft.toolsets);
  const denyTools = normalizeOrderedTokens(draft.denyTools);
  const disabledSkills = normalizeOrderedTokens(draft.disabledSkills);
  const permissions = draft.permissions === "" ? undefined : draft.permissions;
  const categoryPath = validation.categorySegments;

  const sourceTools = [...(source.tools ?? [])];
  const sourceToolsets = [...(source.toolsets ?? [])];
  const sourceDeny = [...(source.deny_tools ?? [])];
  const sourceDisabled = [...(source.skills?.disabled ?? [])];
  const sourceCategory = [...(source.category_path ?? [])];
  const sourcePermissions = mapPermissionsChoice(source.permissions) || undefined;
  const sourceReasoning =
    source.reasoning_effort && isReasoningEffort(source.reasoning_effort)
      ? source.reasoning_effort
      : "";

  const overrides: NonNullable<DuplicateAgentParams["overrides"]> = {};
  if (provider !== (source.provider ?? "")) overrides.provider = provider;
  if (prompt !== (source.prompt ?? "")) overrides.prompt = prompt;
  if (model !== (source.model ?? "")) overrides.model = model || undefined;
  if (command !== (source.command ?? "")) overrides.command = command || undefined;
  if (reasoningEffort !== sourceReasoning) {
    overrides.reasoning_effort = reasoningEffort || undefined;
  }
  if (permissions !== sourcePermissions) overrides.permissions = permissions;
  if (!sameTokenList(tools, sourceTools)) overrides.tools = tools;
  if (!sameTokenList(toolsets, sourceToolsets)) overrides.toolsets = toolsets;
  if (!sameTokenList(denyTools, sourceDeny)) overrides.deny_tools = denyTools;
  if (!sameTokenList(categoryPath, sourceCategory)) {
    overrides.category_path = categoryPath;
  }
  if (!sameTokenList(disabledSkills, sourceDisabled)) {
    overrides.skills = { disabled: disabledSkills };
  }

  const sourceScope: AgentCreateScope = source.origin === "workspace" ? "workspace" : "global";
  const scopeChanged = draft.scope !== sourceScope;
  const hasOverrides = Object.keys(overrides).length > 0;

  return {
    name,
    ...(scopeChanged || draft.scope === "workspace" ? { scope: draft.scope } : {}),
    ...(draft.scope === "workspace" ? { workspace: normalizedWorkspaceId } : {}),
    ...(hasOverrides ? { overrides } : {}),
  };
}

function normalizeOrderedTokens(values: readonly string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const trimmed = value.trim();
    if (trimmed.length === 0 || seen.has(trimmed)) continue;
    seen.add(trimmed);
    normalized.push(trimmed);
  }
  return normalized;
}

function validateToolPatternList(values: readonly string[], label: string): string | null {
  for (const value of values) {
    const trimmed = value.trim();
    if (trimmed.length === 0) return label + " entries cannot be blank.";
    if (!isValidToolPattern(trimmed)) {
      return (
        label +
        " entries must use canonical IDs such as agh__skill_view or namespace wildcards such as agh__task_*."
      );
    }
  }
  return null;
}

function validateToolsetList(values: readonly string[]): string | null {
  for (const value of values) {
    const trimmed = value.trim();
    if (trimmed.length === 0) return "Toolset entries cannot be blank.";
    if (!isValidCanonicalRef(trimmed) || trimmed.includes("*")) {
      return "Toolset entries must use canonical IDs such as agh__catalog.";
    }
  }
  return null;
}

function isValidToolPattern(value: string): boolean {
  if (value.includes("*")) {
    if (value.split("*").length !== 2 || !value.endsWith("*")) return false;
    const prefix = value.slice(0, -1);
    if (prefix.length === 0 || (!prefix.endsWith("_") && !prefix.endsWith("__"))) {
      return false;
    }
    return isValidCanonicalRef(prefix + "x");
  }
  return isValidCanonicalRef(value);
}

function isValidCanonicalRef(value: string): boolean {
  if (!value.includes("__")) return false;
  if (/\s/.test(value)) return false;
  if (value.includes("/") || value.includes("\\")) return false;
  return /^[A-Za-z0-9][A-Za-z0-9_.-]*__[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(value);
}
