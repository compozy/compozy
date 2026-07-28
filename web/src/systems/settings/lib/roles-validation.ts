import type { RoleName, SettingsRolesConfig } from "../types";
import { isReasoningEffortOptionValue, ROLE_ORDER } from "./roles-config";

export interface RoleFieldError {
  /** Deterministic field id shared with the rendered control's `data-field`. */
  id: string;
  message: string;
}

/** Stable id for a scalar role field control (`dream.model`). */
export function roleFieldId(role: RoleName, field: string): string {
  return `${role}.${field}`;
}

/**
 * Stable id for one fallback route (`dream.fallback.0`). A route is a single
 * decision made through one runtime selector, so it carries a single id.
 */
export function fallbackFieldId(role: RoleName, index: number): string {
  return `${role}.fallback.${index}`;
}

/** The role a fallback error belongs to, so a collapsed role can be forced open. */
export function roleFromFieldId(id: string): string {
  return id.split(".")[0] ?? "";
}

/**
 * Validate every fallback route across all roles in visual order (product role
 * order, then entry order). Order is load-bearing: the first element is the
 * deterministic "first invalid field" the save flow focuses. A route needs both
 * a provider and a model; reasoning must be empty or a valid enum value.
 */
export function collectRoleValidationErrors(config: SettingsRolesConfig): RoleFieldError[] {
  const errors: RoleFieldError[] = [];
  const numericFields = [
    ["coordinator", "max_children", config.coordinator.max_children],
    [
      "coordinator",
      "max_active_sessions_per_workspace",
      config.coordinator.max_active_sessions_per_workspace,
    ],
    ["memory_controller", "top_k", config.memory_controller.top_k],
    ["memory_controller", "max_tokens_out", config.memory_controller.max_tokens_out],
  ] as const;
  for (const [role, field, value] of numericFields) {
    const id = roleFieldId(role, field);
    if (!Number.isSafeInteger(value)) {
      errors.push({ id, message: "Enter a safely representable whole number." });
      continue;
    }
    if (value < 1) {
      errors.push({ id, message: "Value must be 1 or greater." });
    }
  }
  for (const role of ROLE_ORDER) {
    config[role].fallback_chain.forEach((entry, index) => {
      const id = fallbackFieldId(role, index);
      if (entry.provider.trim() === "" || entry.model.trim() === "") {
        errors.push({ id, message: "Choose a provider and model." });
        return;
      }
      if (!isReasoningEffortOptionValue(entry.reasoning_effort)) {
        errors.push({ id, message: "Choose a valid reasoning effort." });
      }
    });
  }
  return errors;
}

/** Field-id → message map (first message wins per id). */
export function toRoleErrorMap(errors: RoleFieldError[]): Record<string, string> {
  const map: Record<string, string> = {};
  for (const error of errors) {
    if (!(error.id in map)) {
      map[error.id] = error.message;
    }
  }
  return map;
}

/** The first invalid field in visual order, or null when the draft is valid. */
export function firstRoleFieldError(errors: RoleFieldError[]): RoleFieldError | null {
  return errors[0] ?? null;
}
