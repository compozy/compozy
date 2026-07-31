import type { SettingsMCPServerEntry } from "../types";

export function normalizeMCPAuthScopes(scopes: readonly string[]): string[] {
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const scope of scopes) {
    const value = scope.trim();
    if (!value || seen.has(value)) continue;
    seen.add(value);
    normalized.push(value);
  }
  return normalized;
}

export function mcpScopeEscalationScopes(server: SettingsMCPServerEntry): string[] {
  if (!server.auth_status?.token_present) return [];
  const granted = new Set(normalizeMCPAuthScopes(server.auth_status.scopes ?? []));
  return normalizeMCPAuthScopes(server.auth?.scopes ?? []).filter(scope => !granted.has(scope));
}

export function normalizeMCPRedirectURL(value: string): string {
  return value.trim();
}

/** A manual exchange is a full HTTP(S) redirect URL, never a bare authorization code. */
export function isAbsoluteMCPRedirectURL(value: string): boolean {
  try {
    const url = new URL(normalizeMCPRedirectURL(value));
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}
