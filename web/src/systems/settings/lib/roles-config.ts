import type { ReasoningEffort } from "@/lib/api-contract";

import type { RoleFallbackEntry, RoleName, SettingsRolesConfig } from "../types";

/**
 * Fixed product display order, independent of the API's lexical sort
 * (`GET /api/roles` returns roles sorted by name).
 */
export const ROLE_ORDER: readonly RoleName[] = [
  "coordinator",
  "dream",
  "checkpoint_summary",
  "memory_extractor",
  "auto_title",
  "memory_controller",
];

/** Sentence-case role names for panel headers. */
export const ROLE_LABELS: Record<RoleName, string> = {
  coordinator: "Coordinator",
  dream: "Dream",
  checkpoint_summary: "Checkpoint summary",
  memory_extractor: "Memory extractor",
  auto_title: "Auto title",
  memory_controller: "Memory controller",
};

/** One-line description of what each background role does. */
export const ROLE_DESCRIPTIONS: Record<RoleName, string> = {
  coordinator: "Orchestrates managed sessions.",
  dream: "Consolidates memory while sessions are idle.",
  checkpoint_summary: "Summarizes session checkpoints.",
  memory_extractor: "Extracts durable memories from turns.",
  auto_title: "Names new sessions from their first exchange.",
  memory_controller: "In-process tie-breaker for memory writes.",
};

export type RoleFieldKind = "switch" | "text" | "select" | "number";

export interface RoleFieldDescriptor {
  /** Config field key under `roles.<role>`. */
  key: string;
  label: string;
  description?: string;
  kind: RoleFieldKind;
  placeholder?: string;
  /** Minimum for `number` controls. */
  min?: number;
  /** Render the text control in the mono family (ids, durations, model refs). */
  mono?: boolean;
}

const NO_FIELDS: readonly RoleFieldDescriptor[] = [];

const COORDINATOR_FIELDS: readonly RoleFieldDescriptor[] = [
  {
    key: "ttl",
    label: "Session TTL",
    description: "Coordinator session lifetime.",
    kind: "text",
    placeholder: "2h",
    mono: true,
  },
  {
    key: "max_children",
    label: "Max children",
    description: "Safe-spawn child cap.",
    kind: "number",
    min: 1,
  },
  {
    key: "max_active_sessions_per_workspace",
    label: "Max active sessions",
    description: "Managed-session cap per workspace.",
    kind: "number",
    min: 1,
  },
];

const MEMORY_CONTROLLER_FIELDS: readonly RoleFieldDescriptor[] = [
  {
    key: "timeout",
    label: "Timeout",
    description: "Wall-clock ceiling for the in-process call.",
    kind: "text",
    placeholder: "250ms",
    mono: true,
  },
];

const MEMORY_CONTROLLER_ADVANCED_FIELDS: readonly RoleFieldDescriptor[] = [
  {
    key: "top_k",
    label: "Top K",
    description: "Tie-breaker candidate count.",
    kind: "number",
    min: 1,
  },
  { key: "prompt_version", label: "Prompt version", kind: "text", placeholder: "v1", mono: true },
  { key: "max_tokens_out", label: "Max output tokens", kind: "number", min: 1 },
];

/**
 * Editable policy fields per role. Routing (`enabled`, `agent`, `provider`,
 * `model`, `reasoning_effort`) is not listed here — it is owned by the role
 * header switch, the agent picker, and the runtime selector.
 */
export const ROLE_FIELDS: Record<RoleName, readonly RoleFieldDescriptor[]> = {
  coordinator: COORDINATOR_FIELDS,
  dream: NO_FIELDS,
  checkpoint_summary: NO_FIELDS,
  memory_extractor: NO_FIELDS,
  auto_title: NO_FIELDS,
  memory_controller: MEMORY_CONTROLLER_FIELDS,
};

/** Operator-grade fields that live inside the role's Advanced fold. */
export const ROLE_ADVANCED_FIELDS: Record<RoleName, readonly RoleFieldDescriptor[]> = {
  coordinator: NO_FIELDS,
  dream: NO_FIELDS,
  checkpoint_summary: NO_FIELDS,
  memory_extractor: NO_FIELDS,
  auto_title: NO_FIELDS,
  memory_controller: MEMORY_CONTROLLER_ADVANCED_FIELDS,
};

/** Roles that carry an `agent` key; the in-process controller has no session identity. */
export const ROLE_SUPPORTS_AGENT: Record<RoleName, boolean> = {
  coordinator: true,
  dream: true,
  checkpoint_summary: true,
  memory_extractor: true,
  auto_title: true,
  memory_controller: false,
};

const REASONING_VALUES = [
  "none",
  "minimal",
  "low",
  "medium",
  "high",
  "xhigh",
  "max",
] as const satisfies readonly ReasoningEffort[];

/** Reasoning-effort options ("" = inherit) shared with the AGENT.md enum. */
export const REASONING_OPTIONS: readonly { value: string; label: string }[] = [
  { value: "", label: "Default (inherit)" },
  ...REASONING_VALUES.map(value => ({ value, label: value })),
];

export function isReasoningEffortOptionValue(value: string): boolean {
  return value === "" || (REASONING_VALUES as readonly string[]).includes(value);
}

/**
 * Immutably set one scalar role field, returning a new full section config.
 * The cast is contained here: `ROLE_FIELDS` guarantees `field` is a valid key
 * for `role` and `value`'s runtime kind matches the descriptor.
 */
export function applyRoleFieldEdit(
  config: SettingsRolesConfig,
  role: RoleName,
  field: string,
  value: string | number | boolean
): SettingsRolesConfig {
  return {
    ...config,
    [role]: { ...config[role], [field]: value },
  } as SettingsRolesConfig;
}

/** The three routing keys the runtime selector owns as one decision. */
export interface RoleRuntimeValue {
  provider: string;
  model: string;
  reasoning_effort: string;
}

export const EMPTY_ROLE_RUNTIME: RoleRuntimeValue = {
  provider: "",
  model: "",
  reasoning_effort: "",
};

/**
 * Write provider, model and reasoning effort together. The runtime selector
 * resolves them as one route, so they are never left half-applied.
 */
export function applyRoleRuntimeEdit(
  config: SettingsRolesConfig,
  role: RoleName,
  value: RoleRuntimeValue
): SettingsRolesConfig {
  return {
    ...config,
    [role]: {
      ...config[role],
      provider: value.provider,
      model: value.model,
      reasoning_effort: value.reasoning_effort,
    },
  } as SettingsRolesConfig;
}

/** Clear the route so the role inherits again. */
export function clearRoleRuntime(config: SettingsRolesConfig, role: RoleName): SettingsRolesConfig {
  return applyRoleRuntimeEdit(config, role, EMPTY_ROLE_RUNTIME);
}

export function emptyFallbackEntry(): RoleFallbackEntry {
  return { provider: "", model: "", reasoning_effort: "" };
}

function applyRoleFallbackChain(
  config: SettingsRolesConfig,
  role: RoleName,
  chain: RoleFallbackEntry[]
): SettingsRolesConfig {
  return {
    ...config,
    [role]: { ...config[role], fallback_chain: chain },
  } as SettingsRolesConfig;
}

export function addFallbackEntry(config: SettingsRolesConfig, role: RoleName): SettingsRolesConfig {
  return applyRoleFallbackChain(config, role, [
    ...config[role].fallback_chain,
    emptyFallbackEntry(),
  ]);
}

export function removeFallbackEntry(
  config: SettingsRolesConfig,
  role: RoleName,
  index: number
): SettingsRolesConfig {
  return applyRoleFallbackChain(
    config,
    role,
    config[role].fallback_chain.filter((_entry, entryIndex) => entryIndex !== index)
  );
}

/** Replace one fallback route wholesale — the selector emits all three keys. */
export function setFallbackRuntime(
  config: SettingsRolesConfig,
  role: RoleName,
  index: number,
  value: RoleRuntimeValue
): SettingsRolesConfig {
  const chain = config[role].fallback_chain.map((entry, entryIndex) =>
    entryIndex === index ? { ...entry, ...value } : entry
  );
  return applyRoleFallbackChain(config, role, chain);
}
