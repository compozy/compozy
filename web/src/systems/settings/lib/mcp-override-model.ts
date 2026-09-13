import type { SettingsMCPServerEntry, SettingsMCPServerRequest } from "../types";

export interface MCPOverridePair {
  key: string;
  value: string;
}
export interface MCPOverrideDraft {
  env: MCPOverridePair[];
  headers: MCPOverridePair[];
  url: string;
}
export interface MCPOverrideErrors {
  env?: Record<number, string>;
  headers?: Record<number, string>;
  url?: string;
}

export function toMCPOverrideDraft(entry: SettingsMCPServerEntry): MCPOverrideDraft {
  const pairs = (values: Record<string, string> | undefined) =>
    Object.entries(values ?? {}).map(([key, value]) => ({ key, value }));
  return {
    env: pairs(entry.override?.env),
    headers: pairs(entry.override?.headers),
    url: entry.override?.url ?? "",
  };
}

export function toMCPOverrideRequest(
  name: string,
  draft: MCPOverrideDraft
): SettingsMCPServerRequest {
  const values = (pairs: MCPOverridePair[]) =>
    Object.fromEntries(
      pairs.flatMap(pair => (pair.key.trim() ? [[pair.key.trim(), pair.value]] : []))
    );
  return {
    server: { name, env: values(draft.env), headers: values(draft.headers), url: draft.url.trim() },
  };
}

export function validateMCPOverride(draft: MCPOverrideDraft, transport: string) {
  const errors: MCPOverrideErrors = {};
  for (const field of ["env", "headers"] as const) {
    const rows: Record<number, string> = {};
    const names = new Set<string>();
    draft[field].forEach((pair, index) => {
      const key = pair.key.trim();
      if (!key && !pair.value) return;
      const identity = field === "headers" ? key.toLowerCase() : key;
      if (!key) rows[index] = "Name is required";
      else if (names.has(identity)) rows[index] = "Duplicate name";
      else if (field === "env" && !/^[A-Za-z_][A-Za-z0-9_]*$/.test(key))
        rows[index] = "Invalid environment name";
      else if (field === "headers" && !/^[!#$%&'*+.^_`|~0-9A-Za-z-]+$/.test(key))
        rows[index] = "Invalid header name";
      else if (pair.value.includes("\0") || (field === "headers" && /[\r\n]/.test(pair.value)))
        rows[index] = "Invalid value";
      else if (new TextEncoder().encode(pair.value).length > 8192)
        rows[index] = "Too long (max 8 KB)";
      else if ((transport === "stdio") !== (field === "env"))
        rows[index] = "Unsupported for this transport";
      names.add(identity);
    });
    if (Object.keys(rows).length) errors[field] = rows;
  }
  if (draft.url.trim()) {
    try {
      const url = new URL(draft.url.trim());
      if (
        transport === "stdio" ||
        !["http:", "https:"].includes(url.protocol) ||
        url.username ||
        url.password
      ) {
        errors.url = "Use an HTTP or HTTPS URL without credentials";
      }
    } catch {
      errors.url = "Enter a valid URL";
    }
  }
  return { valid: Object.keys(errors).length === 0, errors };
}
