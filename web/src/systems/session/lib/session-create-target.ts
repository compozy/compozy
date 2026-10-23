import {
  DESTINATION_COPY,
  USER_SCOPE_SELECTOR,
  type FocusedDataScope,
  type WorkspacePayload,
} from "@/systems/workspace";

/** Last All-projects create target in this browser: a workspace id, or `@user` for User. */
export const LAST_SESSION_PROJECT_STORAGE_KEY = "compozy:new-session:last-project";

/** One row of a destination picker. */
export interface SessionTargetOption {
  value: string;
  label: string;
}

/**
 * Project picker rows in switcher order, ending with User — always present, so
 * zero projects still leave one target.
 */
export function sessionProjectOptions(
  workspaces: readonly Pick<WorkspacePayload, "id" | "name">[]
): SessionTargetOption[] {
  return [
    ...workspaces.map(workspace => ({
      value: workspace.id,
      label: workspace.name,
    })),
    { value: USER_SCOPE_SELECTOR, label: DESTINATION_COPY.user },
  ];
}

/**
 * Project prefill (ADR-003): the focused window's binding, then the last
 * project used from All projects, then the first project — User only when no
 * project exists. A candidate that is no longer an option is skipped, so an
 * unregistered id is never offered or submitted.
 */
export function prefillSessionProject(
  options: readonly SessionTargetOption[],
  candidates: { focusedBinding?: string | null; lastUsed?: string | null }
): string {
  const values = new Set(options.map(option => option.value));
  for (const candidate of [candidates.focusedBinding, candidates.lastUsed]) {
    if (candidate && values.has(candidate)) return candidate;
  }
  return options[0]?.value ?? USER_SCOPE_SELECTOR;
}

/** The scope a picker value names. */
export function sessionTargetScope(value: string): FocusedDataScope {
  return value === USER_SCOPE_SELECTOR ? { kind: "user" } : { kind: "project", workspaceId: value };
}

export function readLastSessionProject(): string | null {
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(LAST_SESSION_PROJECT_STORAGE_KEY);
  } catch {
    return null;
  }
}

export function writeLastSessionProject(value: string): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(LAST_SESSION_PROJECT_STORAGE_KEY, value);
  } catch {
    // Storage unavailable: the next create falls back to the first project.
  }
}

/** "Session started in {project} · {profile}" — names both axes the operator may have aggregated. */
export function sessionStartedToast(projectLabel: string, profile: string): string {
  return `Session started in ${projectLabel} · ${profile}`;
}
