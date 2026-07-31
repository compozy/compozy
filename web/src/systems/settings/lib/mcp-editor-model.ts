import type { SettingsMCPServerEntry, SettingsMCPServerRequest } from "../types";

/**
 * Transport-aware MCP editor draft plus serialization/validation. This module
 * mirrors the daemon's transport-conditional field exclusivity client-side:
 *  - stdio keeps command, ordered args, non-secret env, and hybrid secret_env;
 *  - remote HTTP keeps its url and daemon-discovered OAuth policy, and OMITS
 *    command/args/env/secret_env.
 * Secret values and bindings are write-only. Read responses expose configured
 * presence only; unchanged bindings are retained through `preserve_secrets`.
 * Field-error strings mirror `internal/config/mcp_server_validation.go` so the
 * client message matches the daemon rejection.
 */

export type MCPTransport = "stdio" | "http";
export type MCPAuthRegistration = "auto" | "pre_registered";
export type MCPSecretMode = "preserve" | "typed" | "ref";

export type MCPEnvPair = {
  key: string;
  value: string;
  /** Presence-only read state; the prior value is never returned. */
  existing?: boolean;
  /** Exact prior key eligible for same-target preservation. */
  originalKey?: string;
  /** Distinguishes an explicit value (including empty) from an untouched hidden value. */
  valueChanged?: boolean;
};

export interface MCPSecretBinding {
  mode: MCPSecretMode;
  /** Presence-only read state; the bound identifier is never available here. */
  existing: boolean;
  /** Write-only replacement value; sent via `secret_values`, never echoed back. */
  typedValue: string;
  /** An explicit `vault:mcp/**` ref selected from the separate Vault inventory. */
  vaultRef: string;
}

export interface MCPSecretEnvEntry {
  key: string;
  /** Original presence-bearing key; preservation is invalid after a rename. */
  originalKey?: string;
  binding: MCPSecretBinding;
}

export interface MCPOAuthDraft {
  enabled: boolean;
  registration: MCPAuthRegistration;
  issuerUrl: string;
  clientId: string;
  /** Space-separated scopes in the input; serialized to a string[]. */
  scopes: string;
}

export interface MCPDraft {
  name: string;
  transport: MCPTransport;
  // stdio fields
  command: string;
  args: string[];
  env: MCPEnvPair[];
  secretEnv: MCPSecretEnvEntry[];
  // remote fields
  url: string;
  oauth: MCPOAuthDraft;
}

export interface MCPDraftErrors {
  name?: string;
  command?: string;
  url?: string;
  clientId?: string;
  issuerUrl?: string;
  remoteFields?: string;
  /** Per-row plain-env errors, keyed by the draft `env` index. */
  env?: Record<number, string>;
  /** Per-row stdio secret-env binding errors, keyed by the draft `secretEnv` index. */
  secretEnv?: Record<number, string>;
}

export interface MCPDraftValidation {
  valid: boolean;
  errors: MCPDraftErrors;
}

function emptyBinding(): MCPSecretBinding {
  return { mode: "typed", existing: false, typedValue: "", vaultRef: "" };
}

function emptyOAuth(): MCPOAuthDraft {
  return {
    enabled: false,
    registration: "auto",
    issuerUrl: "",
    clientId: "",
    scopes: "",
  };
}

export function emptyDraft(transport: MCPTransport = "stdio"): MCPDraft {
  return {
    name: "",
    transport,
    command: "",
    args: [],
    env: [],
    secretEnv: [],
    url: "",
    oauth: emptyOAuth(),
  };
}

/**
 * Normalize a draft when its transport changes so the newly visible form starts
 * completable and no prior-transport field can leak or serialize. Crossing the
 * stdio↔remote boundary clears the other side's fields; a same-transport
 * selection is a no-op.
 */
export function withTransport(draft: MCPDraft, transport: MCPTransport): MCPDraft {
  if (draft.transport === transport) return draft;
  if (transport === "stdio") {
    return { ...draft, transport, url: "", oauth: emptyOAuth() };
  }
  return { ...draft, transport, command: "", args: [], env: [], secretEnv: [] };
}

function normalizeTransport(value: string | undefined): MCPTransport {
  return value === "http" ? "http" : "stdio";
}

function configuredBinding(configured: boolean): MCPSecretBinding {
  if (!configured) return emptyBinding();
  return { mode: "preserve", existing: true, typedValue: "", vaultRef: "" };
}

/** Seed an editor draft from an existing server entry (never reflects secret values). */
export function toDraft(entry: SettingsMCPServerEntry): MCPDraft {
  const transport = normalizeTransport(entry.transport);
  const env: MCPEnvPair[] = (entry.env_keys ?? []).map(key => ({
    key,
    value: "",
    existing: true,
    originalKey: key,
    valueChanged: false,
  }));
  const secretEnv: MCPSecretEnvEntry[] = (entry.secret_env_keys ?? []).map(key => ({
    key,
    originalKey: key,
    binding: configuredBinding(true),
  }));
  const auth = entry.auth ?? null;
  const oauth: MCPOAuthDraft = auth
    ? {
        enabled: true,
        registration: auth.registration === "pre_registered" ? "pre_registered" : "auto",
        issuerUrl: auth.issuer_url ?? "",
        clientId: auth.client_id ?? "",
        scopes: (auth.scopes ?? []).join(" "),
      }
    : emptyOAuth();
  return {
    name: entry.name,
    transport,
    command: entry.command ?? "",
    args: [...(entry.args ?? [])],
    env,
    secretEnv,
    url: entry.url ?? "",
    oauth,
  };
}

/** Remove effective-source presence when an edit creates a different-scope override. */
export function withoutMCPSecretPreservation(draft: MCPDraft): MCPDraft {
  const withoutPreservation = (binding: MCPSecretBinding): MCPSecretBinding => ({
    ...binding,
    existing: false,
    mode: binding.mode === "preserve" ? "typed" : binding.mode,
  });
  return {
    ...draft,
    env: draft.env.map(entry => ({ ...entry, originalKey: undefined })),
    secretEnv: draft.secretEnv.map(entry => ({
      ...entry,
      originalKey: undefined,
      binding: withoutPreservation(entry.binding),
    })),
  };
}

type ServerBody = SettingsMCPServerRequest["server"];
type AuthBody = NonNullable<ServerBody["auth"]>;

type SecretResolution =
  | { kind: "preserve" }
  | { kind: "ref"; ref: string }
  | { kind: "typed"; value: string }
  | { kind: "none" };

/**
 * The single precedence rule for what a hybrid secret binding contributes to a
 * request, shared by stdio secret_env serialization and validation so the two
 * can never drift. Each explicit mode has one output: preservation by presence,
 * a selected Vault ref, or a typed write-only value.
 */
function resolveSecretBinding(binding: MCPSecretBinding): SecretResolution {
  if (binding.mode === "preserve") {
    return binding.existing ? { kind: "preserve" } : { kind: "none" };
  }
  if (binding.mode === "ref" && binding.vaultRef.trim()) {
    return { kind: "ref", ref: binding.vaultRef.trim() };
  }
  if (binding.mode === "typed" && binding.typedValue !== "") {
    return { kind: "typed", value: binding.typedValue };
  }
  return { kind: "none" };
}

function resolveSecretEnvBinding(entry: MCPSecretEnvEntry): SecretResolution {
  const resolved = resolveSecretBinding(entry.binding);
  if (
    resolved.kind === "preserve" &&
    entry.originalKey !== undefined &&
    entry.key.trim() !== entry.originalKey.trim()
  ) {
    return { kind: "none" };
  }
  return resolved;
}

function collectEnv(pairs: MCPEnvPair[]): {
  values: Record<string, string>;
  preserved: string[];
} {
  const env: Record<string, string> = {};
  const preserved: string[] = [];
  for (const pair of pairs) {
    const key = pair.key.trim();
    if (!key) continue;
    if (
      pair.existing &&
      !pair.valueChanged &&
      pair.originalKey !== undefined &&
      key === pair.originalKey.trim()
    ) {
      preserved.push(key);
    } else {
      env[key] = pair.value;
    }
  }
  return { values: env, preserved };
}

/**
 * Serialize a draft into the PUT body. stdio emits command/args/env + explicitly
 * selected refs on `server.secret_env`, unchanged names under `preserve_secrets`,
 * and typed replacements under `secret_values.secret_env`;
 * remote emits url + optional discovered OAuth policy and NEVER
 * command/args/env/secret_env. The daemon computes canonical scope-qualified
 * refs from typed stdio values.
 */
export function toRequest(draft: MCPDraft): SettingsMCPServerRequest {
  const name = draft.name.trim();
  if (draft.transport === "stdio") {
    const server: ServerBody = { name, transport: "stdio", command: draft.command.trim() };
    const args = draft.args.flatMap(arg => {
      const trimmed = arg.trim();
      return trimmed ? [trimmed] : [];
    });
    if (args.length > 0) server.args = args;
    const env = collectEnv(draft.env);
    if (Object.keys(env.values).length > 0) server.env = env.values;
    const keptRefs: Record<string, string> = {};
    const preservedSecrets: string[] = [];
    const typedSecrets: Record<string, string> = {};
    for (const entry of draft.secretEnv) {
      const key = entry.key.trim();
      if (!key) continue;
      const resolved = resolveSecretEnvBinding(entry);
      if (resolved.kind === "preserve") preservedSecrets.push(key);
      else if (resolved.kind === "ref") keptRefs[key] = resolved.ref;
      else if (resolved.kind === "typed") typedSecrets[key] = resolved.value;
    }
    if (Object.keys(keptRefs).length > 0) server.secret_env = keptRefs;
    const request: SettingsMCPServerRequest = { server };
    if (env.preserved.length > 0) request.preserve_env = env.preserved;
    if (preservedSecrets.length > 0) {
      request.preserve_secrets = { secret_env: preservedSecrets };
    }
    if (Object.keys(typedSecrets).length > 0) {
      request.secret_values = { secret_env: typedSecrets };
    }
    return request;
  }
  const server: ServerBody = { name, transport: draft.transport, url: draft.url.trim() };
  const request: SettingsMCPServerRequest = { server };
  if (draft.oauth.enabled) {
    const auth: AuthBody = { registration: draft.oauth.registration };
    if (draft.oauth.issuerUrl.trim()) auth.issuer_url = draft.oauth.issuerUrl.trim();
    if (draft.oauth.clientId.trim()) auth.client_id = draft.oauth.clientId.trim();
    const scopes = draft.oauth.scopes.split(/\s+/).filter(Boolean);
    if (scopes.length > 0) auth.scopes = scopes;
    server.auth = auth;
  }
  return request;
}

function hasRemoteForbiddenFields(draft: MCPDraft): string | undefined {
  if (draft.command.trim()) return "Command is only valid for stdio transport";
  if (draft.args.some(arg => arg.trim())) return "Args is only valid for stdio transport";
  if (draft.env.some(pair => pair.key.trim())) return "Env is only valid for stdio transport";
  if (draft.secretEnv.some(entry => entry.key.trim())) {
    return "Secret env is only valid for stdio transport";
  }
  return undefined;
}

/**
 * A non-empty stdio secret key whose binding resolves to nothing (no typed
 * value, selected Vault ref, or preserved existing ref) would be silently
 * dropped from the save, so it is a per-row error. Blank-key rows are pristine
 * (matching serialization, which trims them out) and never flagged.
 */
function validateStdioSecretEnv(secretEnv: MCPSecretEnvEntry[]): Record<number, string> {
  const errors: Record<number, string> = {};
  secretEnv.forEach((entry, index) => {
    if (entry.key.trim().length === 0) return;
    if (resolveSecretEnvBinding(entry).kind === "none") {
      errors[index] = "Enter a value or select a Vault reference";
    }
  });
  return errors;
}

function validateStdioEnv(env: MCPEnvPair[]): Record<number, string> {
  const errors: Record<number, string> = {};
  env.forEach((entry, index) => {
    const key = entry.key.trim();
    if (!key || !entry.existing || entry.valueChanged) return;
    if (entry.originalKey === undefined || key !== entry.originalKey.trim()) {
      errors[index] = "Enter a value after changing this existing key or target";
    }
  });
  return errors;
}

/** Client-side mirror of the daemon's transport-conditional validation. */
export function validateDraft(draft: MCPDraft): MCPDraftValidation {
  const errors: MCPDraftErrors = {};
  if (draft.name.trim().length === 0) errors.name = "Name is required";

  if (draft.transport === "stdio") {
    if (draft.command.trim().length === 0) errors.command = "Command is required";
    const envErrors = validateStdioEnv(draft.env);
    if (Object.keys(envErrors).length > 0) errors.env = envErrors;
    const secretEnvErrors = validateStdioSecretEnv(draft.secretEnv);
    if (Object.keys(secretEnvErrors).length > 0) errors.secretEnv = secretEnvErrors;
  } else {
    if (draft.url.trim().length === 0) {
      errors.url = `URL is required for ${draft.transport} transport`;
    }
    const forbidden = hasRemoteForbiddenFields(draft);
    if (forbidden) errors.remoteFields = forbidden;
    if (draft.oauth.enabled && draft.oauth.registration === "pre_registered") {
      if (draft.oauth.clientId.trim().length === 0) {
        errors.clientId = "Client ID is required for OAuth";
      }
      if (draft.oauth.issuerUrl.trim().length === 0) errors.issuerUrl = "Issuer URL is required";
    }
  }

  return { valid: Object.keys(errors).length === 0, errors };
}
