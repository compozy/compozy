import { isReasoningEffort, type ReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";

import type { AgentAvatarAppearance } from "@compozy/ui";

import { joinAgentCategorySegments } from "./agent-category";
import { normalizeRuntimeSpeed, runtimeACPSelections } from "./agent-effective-runtime";
import { agentCreateAppearance, agentCreateSlug, appearancePayload } from "./agent-create-identity";
import { validateToolPatternList, validateToolsetList } from "./agent-create-tool-patterns";
import { agentAvatarAppearance, GLOBAL_SCOPE } from "./agent-instance";
import type { AgentPayload, CreateAgentParams, DuplicateAgentParams } from "../types";
import {
  runtimeACPSelectionsEqual,
  type RuntimeACPOptionSelection,
  type RuntimeProviderOption,
} from "@/systems/runtime";

const agentNameMaxLength = 106;
const agentNamePattern = /^[a-z][a-z0-9_-]{0,105}$/;
const invalidAgentNameMessage = `Start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and use at most ${agentNameMaxLength} characters.`;

type AgentAppearanceInput = Parameters<typeof agentAvatarAppearance>[0];

export type AgentCreateScope = CreateAgentParams["scope"];
export type AgentCreatePermission = NonNullable<CreateAgentParams["agent"]["permissions"]>;
export type AgentCreatePermissionChoice = "" | AgentCreatePermission;

export { agentCreateAppearance, agentCreateSlug, agentSlugFor } from "./agent-create-identity";

export type AgentCreateKind = "on_demand" | "stays_on";

/** `Decide later`: the agent is created with no instance. A place is otherwise a scope selector. */
export const AGENT_PLACE_LATER = "later";

export interface AgentCreateDialogDraft {
  scope: AgentCreateScope;
  /** The Name field: what every surface shows. */
  displayName: string;
  description: string;
  /** The Short name field: empty derives the slug from `displayName`. */
  name: string;
  /** `null` until someone picks: the face follows the slug. */
  appearance: AgentAvatarAppearance | null;
  kind: AgentCreateKind;
  /** Where a Stays on agent starts: a workspace id, `@user`, or `later`. */
  place: string;
  categoryPath: string;
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort | "";
  speed: RuntimeSpeed | "";
  /** Typed ACP overrides preserved when duplicating or prefilling an agent. */
  acpOptions?: RuntimeACPOptionSelection[];
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
  "categoryPath",
  "tools",
  "toolsets",
  "denyTools",
] as const;

export type AgentCreateFieldKey =
  | "name"
  | "categoryPath"
  | "provider"
  | "reasoningEffort"
  | "prompt"
  | "tools"
  | "toolsets"
  | "denyTools";

export function createDefaultAgentCreateDraft(
  hasActiveWorkspace: boolean,
  activeWorkspaceId?: string | null
): AgentCreateDialogDraft {
  return {
    scope: hasActiveWorkspace ? "workspace" : "global",
    displayName: "",
    description: "",
    name: "",
    appearance: null,
    kind: "on_demand",
    place: activeWorkspaceId ?? GLOBAL_SCOPE,
    categoryPath: "",
    provider: "",
    model: "",
    reasoningEffort: "",
    speed: "",
    acpOptions: undefined,
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
    speed: "",
    acpOptions: undefined,
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
    return { segments: [], error: "Group can't contain backslashes." };
  }

  const rawSegments = trimmed.split("/");
  const segments: string[] = [];
  for (const rawSegment of rawSegments) {
    const segment = rawSegment.trim();
    if (segment.length === 0) {
      return { segments: [], error: "Group can't contain empty parts between slashes." };
    }
    if (segment === "." || segment === "..") {
      return { segments: [], error: "Group can't contain . or .. parts." };
    }
    if (segment.includes("\\") || segment.includes("/")) {
      return { segments: [], error: "Group parts can't contain path separators." };
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
  const name = agentCreateSlug(draft);
  if (draft.displayName.trim().length === 0 && draft.name.trim().length === 0) {
    fields.name = "Enter an agent name.";
  } else if (name.length === 0) {
    fields.name = "Add at least one letter or number to the name.";
  } else if (!agentNamePattern.test(name)) {
    fields.name = invalidAgentNameMessage;
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
  context: AgentCreateValidationContext,
  avatarColors: readonly string[] = []
): CreateAgentParams | null {
  const validation = validateAgentCreateDraft(draft, context);
  if (!validation.canSubmit) return null;
  const normalizedWorkspaceId = workspaceId?.trim() ?? "";
  if (draft.scope === "workspace" && normalizedWorkspaceId.length === 0) {
    return null;
  }

  const name = agentCreateSlug(draft);
  const displayName = draft.displayName.trim();
  const description = draft.description.trim();
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
      kind: draft.kind,
      appearance: appearancePayload(agentCreateAppearance(draft), avatarColors),
      ...(displayName.length > 0 ? { display_name: displayName } : {}),
      ...(description.length > 0 ? { description } : {}),
      ...(provider.length > 0 ? { provider } : {}),
      ...(command.length > 0 ? { command } : {}),
      ...(model.length > 0 ? { model } : {}),
      ...(reasoningEffort !== "" ? { reasoning_effort: reasoningEffort } : {}),
      ...(draft.speed !== "" ? { speed: draft.speed } : {}),
      ...(draft.acpOptions !== undefined ? { acp_options: [...draft.acpOptions] } : {}),
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
    displayName: "",
    description: agent.description ?? "",
    name: "",
    appearance: agentAvatarAppearance(agent.appearance as AgentAppearanceInput, agent.name),
    kind: agent.kind === "stays_on" ? "stays_on" : "on_demand",
    place: AGENT_PLACE_LATER,
    categoryPath: joinAgentCategorySegments(agent.category_path ?? []),
    provider: agent.provider ?? "",
    model: agent.model ?? "",
    reasoningEffort: reasoning,
    speed: normalizeRuntimeSpeed(agent.speed),
    acpOptions: runtimeACPSelections(agent.acp_options),
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

/** Whether two drafts hold the same answers in every field. */
export function agentCreateDraftsEqual(
  left: AgentCreateDialogDraft,
  right: AgentCreateDialogDraft
): boolean {
  const { appearance, acpOptions, tools, toolsets, denyTools, disabledSkills, ...text } = left;
  return (
    (Object.keys(text) as (keyof typeof text)[]).every(field => text[field] === right[field]) &&
    JSON.stringify(appearance) === JSON.stringify(right.appearance) &&
    runtimeACPSelectionsEqual(acpOptions, right.acpOptions) &&
    sameTokenList(tools, right.tools) &&
    sameTokenList(toolsets, right.toolsets) &&
    sameTokenList(denyTools, right.denyTools) &&
    sameTokenList(disabledSkills, right.disabledSkills)
  );
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

  const name = agentCreateSlug(draft);
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
  const sourceACPOptions = runtimeACPSelections(source.acp_options);
  const sourceSpeed = normalizeRuntimeSpeed(source.speed);

  const overrides: NonNullable<DuplicateAgentParams["overrides"]> = {};
  const displayName = draft.displayName.trim();
  const description = draft.description.trim();
  if (displayName.length > 0) overrides.display_name = displayName;
  if (description !== (source.description ?? "")) overrides.description = description;
  if (provider !== (source.provider ?? "")) overrides.provider = provider;
  if (prompt !== (source.prompt ?? "")) overrides.prompt = prompt;
  if (model !== (source.model ?? "")) overrides.model = model || undefined;
  if (command !== (source.command ?? "")) overrides.command = command || undefined;
  if (reasoningEffort !== sourceReasoning) {
    overrides.reasoning_effort = reasoningEffort || undefined;
  }
  if (draft.speed !== sourceSpeed) {
    if (draft.speed) overrides.speed = draft.speed;
    else overrides.clear_speed = true;
  }
  if (!runtimeACPSelectionsEqual(draft.acpOptions, sourceACPOptions)) {
    if (draft.acpOptions) overrides.acp_options = [...draft.acpOptions];
    else overrides.clear_acp_options = true;
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
    ...(draft.kind !== (source.kind ?? "on_demand") ? { kind: draft.kind } : {}),
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
