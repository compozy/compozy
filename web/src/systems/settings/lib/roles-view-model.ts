import type {
  RoleFallbackEntry,
  RoleName,
  RoleResolutionMode,
  RoleStatus,
  SettingsRolesConfig,
} from "../types";
import {
  ROLE_ADVANCED_FIELDS,
  ROLE_DESCRIPTIONS,
  ROLE_FIELDS,
  ROLE_LABELS,
  ROLE_ORDER,
  ROLE_SUPPORTS_AGENT,
  type RoleFieldDescriptor,
  type RoleRuntimeValue,
} from "./roles-config";

/**
 * Compact header pill states. `off` is carried by the header switch, and
 * `catalog` mode by the routed agent name, so neither takes a pill.
 */
export type RoleBadge = "builtin" | "inherit";

export interface RoleViewModel {
  role: RoleName;
  label: string;
  description: string;
  enabled: boolean;
  /** Whether this role has an `agent` key at all. */
  supportsAgent: boolean;
  /** Draft agent override; empty means the role default. */
  agent: string;
  /** Draft provider/model/reasoning route, as the runtime selector reads it. */
  runtime: RoleRuntimeValue;
  /** Whether any part of the route is pinned, which is what "Clear override" acts on. */
  hasRuntimeOverride: boolean;
  /** Editable policy fields shown in the role body. */
  fields: readonly RoleFieldDescriptor[];
  /** Operator-grade fields shown inside the role's Advanced fold. */
  advancedFields: readonly RoleFieldDescriptor[];
  /** Draft values for every scalar field, flattened at this read boundary. */
  values: Record<string, string | number | boolean>;
  /** Draft fallback routes for this role. */
  fallbackChain: RoleFallbackEntry[];
  /**
   * Effective (projected) values for routing fields — `null` means "resolves at
   * invocation" and is never replaced with a fabricated default.
   */
  effective: Record<string, string | null>;
  badges: RoleBadge[];
  resolutionMode: RoleResolutionMode;
  resolutionLine: string;
  /** Projected route for the collapsed header; `null` when nothing is pinned. */
  routeSummary: string | null;
  hasDiagnostics: boolean;
  /** The raw projection — provenance, diagnostics, and null truth all read from it. */
  status: RoleStatus;
}

function computeBadges(status: RoleStatus): RoleBadge[] {
  if (status.resolution_mode === "builtin") {
    return ["builtin"];
  }
  if (status.resolution_mode === "inherit") {
    return ["inherit"];
  }
  return [];
}

/**
 * The route as the daemon projects it, for the collapsed header. Only pinned
 * values are shown; a role that resolves at invocation gets no chip rather than
 * an invented one.
 */
export function computeRouteSummary(status: RoleStatus): string | null {
  const parts = [status.provider, status.model].filter(
    (part): part is string => typeof part === "string" && part.trim() !== ""
  );
  return parts.length > 0 ? parts.join(" · ") : null;
}

/** Inherit-mode roles resolve only at invocation; the surface says so rather than guessing. */
export function computeResolutionLine(status: RoleStatus): string {
  switch (status.resolution_mode) {
    case "inherit":
      return "Resolves at invocation.";
    case "builtin":
      return status.agent ? `Built-in · ${status.agent}` : "Built-in identity.";
    case "catalog":
      return status.agent ? `Agent · ${status.agent}` : "Routed to a catalog agent.";
    default:
      return "Resolves at invocation.";
  }
}

function buildEffective(status: RoleStatus): Record<string, string | null> {
  const effective: Record<string, string | null> = {
    agent: status.agent,
    provider: status.provider,
    model: status.model,
    reasoning_effort: status.reasoning_effort,
  };
  if (status.timeout != null) {
    effective.timeout = status.timeout;
  }
  return effective;
}

function buildRoleViewModel(
  role: RoleName,
  status: RoleStatus,
  config: SettingsRolesConfig
): RoleViewModel {
  const fields = ROLE_FIELDS[role];
  const advancedFields = ROLE_ADVANCED_FIELDS[role];
  // Flatten the union-typed role config into a descriptor-keyed value map. The
  // cast is contained to this read boundary; only scalar keys are read (never
  // `fallback_chain`), so widening through `unknown` is safe.
  const roleConfig = config[role] as unknown as Record<string, string | number | boolean>;
  const values: Record<string, string | number | boolean> = {};
  for (const field of [...fields, ...advancedFields]) {
    values[field.key] = roleConfig[field.key];
  }
  const runtime: RoleRuntimeValue = {
    provider: String(roleConfig.provider ?? ""),
    model: String(roleConfig.model ?? ""),
    reasoning_effort: String(roleConfig.reasoning_effort ?? ""),
  };
  const supportsAgent = ROLE_SUPPORTS_AGENT[role];
  return {
    role,
    label: ROLE_LABELS[role],
    description: ROLE_DESCRIPTIONS[role],
    enabled: Boolean(roleConfig.enabled),
    supportsAgent,
    agent: supportsAgent ? String(roleConfig.agent ?? "") : "",
    runtime,
    hasRuntimeOverride: Object.values(runtime).some(value => value.trim() !== ""),
    fields,
    advancedFields,
    values,
    fallbackChain: config[role].fallback_chain,
    effective: buildEffective(status),
    badges: computeBadges(status),
    resolutionMode: status.resolution_mode,
    resolutionLine: computeResolutionLine(status),
    routeSummary: computeRouteSummary(status),
    hasDiagnostics: status.diagnostics.length > 0,
    status,
  };
}

/**
 * Join the read-only projection with the editable draft into the six role view
 * models in fixed product order (`ROLE_ORDER`), independent of the API's lexical
 * ordering. A role absent from the projection is skipped rather than fabricated.
 */
export function buildRolesViewModel(
  statuses: readonly RoleStatus[],
  config: SettingsRolesConfig
): RoleViewModel[] {
  const byRole = new Map(statuses.map(status => [status.role, status]));
  const models: RoleViewModel[] = [];
  for (const role of ROLE_ORDER) {
    const status = byRole.get(role);
    if (status) {
      models.push(buildRoleViewModel(role, status, config));
    }
  }
  return models;
}
