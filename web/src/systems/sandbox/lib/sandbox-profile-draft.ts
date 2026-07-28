import type { SettingsSandboxEntry, SettingsSandboxRequest } from "@/systems/settings";

type Profile = SettingsSandboxRequest["profile"];
type NetworkProfile = NonNullable<Profile["network"]>;
type DaytonaProfile = NonNullable<Profile["daytona"]>;

/** One `KEY=value` row; `secret_env` values are vault/env references, not secrets. */
export interface SandboxEnvPair {
  key: string;
  value: string;
}

export interface SandboxDraft {
  name: string;
  backend: string;
  sync_mode: string;
  persistence: string;
  runtime_root: string;
  env: SandboxEnvPair[];
  /** `KEY` → `env:NAME` / `vault:sandbox/...` reference. Never a plaintext secret. */
  secretEnv: SandboxEnvPair[];
  network: NetworkProfile;
  daytona: DaytonaProfile;
  /** Profile keys this editor does not model, round-tripped untouched. */
  preserved: Omit<
    Profile,
    | "backend"
    | "sync_mode"
    | "persistence"
    | "runtime_root"
    | "env"
    | "secret_env"
    | "network"
    | "daytona"
  >;
}

function toPairs(map: Record<string, string> | undefined): SandboxEnvPair[] {
  if (!map) return [];
  return Object.entries(map).map(([key, value]) => ({ key, value }));
}

function fromPairs(pairs: readonly SandboxEnvPair[]): Record<string, string> | undefined {
  const entries: [string, string][] = [];
  for (const pair of pairs) {
    const key = pair.key.trim();
    if (key.length > 0) entries.push([key, pair.value.trim()]);
  }
  if (entries.length === 0) return undefined;
  return Object.fromEntries(entries);
}

/** Mirrors `vault.EnvNamePattern` (`internal/vault/types.go`). */
const ENV_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_]*$/;
const SECRET_LIKE_ENV_NEEDLES = [
  "SECRET",
  "TOKEN",
  "PASSWORD",
  "PASSWD",
  "API_KEY",
  "APIKEY",
  "PRIVATE_KEY",
  "PRIVATEKEY",
  "AUTHORIZATION",
  "BEARER",
  "CREDENTIAL",
] as const;
/** Mirrors `vault.vaultRefPattern` narrowed to the `sandbox` namespace. */
const SANDBOX_VAULT_REF_PATTERN =
  /^vault:sandbox\/[a-z0-9][a-z0-9_.-]*(?:\/[A-Za-z0-9][A-Za-z0-9_.-]*)*$/;

/**
 * Validates one `secret_env` row against the daemon's reference contract.
 *
 * `vault.ValidateSecretEnvMap` rejects anything that is not an `env:` or
 * namespaced `vault:sandbox/...` reference, so a plaintext value must be caught
 * here rather than travelling to a PUT that will fail — or worse, persisting a
 * credential into a profile that only ever stores pointers.
 */
export function validateSandboxSecretEnvRef(pair: SandboxEnvPair): string | null {
  const key = pair.key.trim();
  const ref = pair.value.trim();
  if (key.length === 0 && ref.length === 0) return null;
  if (key.length === 0) return "Enter the environment variable name.";
  if (!ENV_NAME_PATTERN.test(key)) return `${key} is not a valid environment variable name.`;
  if (ref.length === 0) return `Enter an env: or vault:sandbox/… reference for ${key}.`;
  if (ref.startsWith("env:")) {
    const envName = ref.slice("env:".length).trim();
    return ENV_NAME_PATTERN.test(envName)
      ? null
      : `${ref} is not a valid env reference for ${key}.`;
  }
  if (ref.startsWith("vault:")) {
    return SANDBOX_VAULT_REF_PATTERN.test(ref)
      ? null
      : `${key} must use vault:sandbox/<path>; ${ref} is outside the sandbox namespace.`;
  }
  return `${key} must reference a secret with env:NAME or vault:sandbox/<path> — never a literal value.`;
}

/** Mirrors `vault.ValidateNonSecretEnvMap` before a settings replace request. */
export function validateSandboxEnvPair(pair: SandboxEnvPair): string | null {
  const key = pair.key.trim();
  const value = pair.value.trim();
  if (key.length === 0 && value.length === 0) return null;
  if (key.length === 0) return "Enter the environment variable name.";
  if (!ENV_NAME_PATTERN.test(key)) return `${key} is not a valid environment variable name.`;
  if (isSecretLikeEnvName(key)) {
    return `${key} must move secret-like values to secret environment.`;
  }
  return null;
}

function isSecretLikeEnvName(name: string): boolean {
  const normalized = name.trim().toUpperCase();
  if (
    normalized.endsWith("_URL") ||
    normalized.endsWith("_URI") ||
    normalized.endsWith("_PATH") ||
    normalized.endsWith("_FILE") ||
    normalized.endsWith("_DIR")
  ) {
    return false;
  }
  return SECRET_LIKE_ENV_NEEDLES.some(needle => normalized.includes(needle));
}

/** Every `env` row error, in draft order. Empty when the draft is writable. */
export function sandboxEnvErrors(draft: SandboxDraft): Record<number, string> {
  const errors: Record<number, string> = {};
  draft.env.forEach((pair, index) => {
    const error = validateSandboxEnvPair(pair);
    if (error) errors[index] = error;
  });
  return errors;
}

/** Every `secret_env` row error, in draft order. Empty when the draft is writable. */
export function sandboxSecretEnvErrors(draft: SandboxDraft): Record<number, string> {
  const errors: Record<number, string> = {};
  draft.secretEnv.forEach((pair, index) => {
    const error = validateSandboxSecretEnvRef(pair);
    if (error) errors[index] = error;
  });
  return errors;
}

export function emptySandboxDraft(): SandboxDraft {
  return {
    name: "",
    backend: "local",
    sync_mode: "",
    persistence: "",
    runtime_root: "",
    env: [],
    secretEnv: [],
    network: {},
    daytona: {},
    preserved: {},
  };
}

export function toSandboxDraft(entry: SettingsSandboxEntry): SandboxDraft {
  const {
    backend,
    sync_mode,
    persistence,
    runtime_root,
    env,
    secret_env,
    network,
    daytona,
    ...preserved
  } = entry.profile;
  return {
    name: entry.name,
    backend,
    sync_mode: sync_mode ?? "",
    persistence: persistence ?? "",
    runtime_root: runtime_root ?? "",
    env: toPairs(env),
    secretEnv: toPairs(secret_env),
    network: network ?? {},
    daytona: daytona ?? {},
    preserved,
  };
}

/**
 * Builds the full-replacement PUT body.
 *
 * `PutSettingsSandboxRequest` has no preservation channel, so anything the
 * editor does not model has to be round-tripped explicitly. Daytona parameters
 * are dropped when the backend leaves `daytona`: keeping them would persist a
 * cloud configuration for a profile that no longer runs there.
 */
export function toSandboxRequest(draft: SandboxDraft): SettingsSandboxRequest {
  const backend = draft.backend.trim();
  const profile: Profile = { backend, ...draft.preserved };
  if (draft.sync_mode.trim()) profile.sync_mode = draft.sync_mode.trim();
  if (draft.persistence.trim()) profile.persistence = draft.persistence.trim();
  if (draft.runtime_root.trim()) profile.runtime_root = draft.runtime_root.trim();

  const env = fromPairs(draft.env.filter(pair => validateSandboxEnvPair(pair) === null));
  if (env) profile.env = env;
  // Only reference-shaped rows are serialized: a literal value here would be a
  // credential persisted into a profile that stores pointers only.
  const secretEnv = fromPairs(
    draft.secretEnv.filter(pair => validateSandboxSecretEnvRef(pair) === null)
  );
  if (secretEnv) profile.secret_env = secretEnv;

  const network = compactNetwork(draft.network, backend);
  if (network) profile.network = network;

  if (backend === "daytona") {
    const daytona = compactDaytona(draft.daytona);
    if (daytona) profile.daytona = daytona;
  }

  return { profile };
}

function compactNetwork(network: NetworkProfile, backend: string): NetworkProfile | undefined {
  // Daytona only applies public ingress. Its alpha provider explicitly cannot
  // enforce outbound or hostname allow/deny policy, so never preserve or send
  // those claims through a full replacement request.
  if (backend === "daytona") {
    return network.allow_public_ingress ? { allow_public_ingress: true } : undefined;
  }

  const allowList = (network.allow_list ?? []).filter(entry => entry.trim().length > 0);
  const denyList = (network.deny_list ?? []).filter(entry => entry.trim().length > 0);
  const compact: NetworkProfile = {};
  if (network.allow_public_ingress) compact.allow_public_ingress = true;
  if (network.allow_outbound) compact.allow_outbound = true;
  if (network.required) compact.required = true;
  if (allowList.length > 0) compact.allow_list = allowList;
  if (denyList.length > 0) compact.deny_list = denyList;
  return Object.keys(compact).length > 0 ? compact : undefined;
}

function compactDaytona(daytona: DaytonaProfile): DaytonaProfile | undefined {
  const compact: DaytonaProfile = {};
  for (const [key, value] of Object.entries(daytona)) {
    if (typeof value === "string" && value.trim().length > 0) {
      compact[key as keyof DaytonaProfile] = value.trim();
    }
  }
  return Object.keys(compact).length > 0 ? compact : undefined;
}
