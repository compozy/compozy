import type {
  ProviderAuthMode,
  ProviderCredentialSlotDraft,
  ProviderDraft,
  SettingsProviderCredentialSlotRequest,
  SettingsProviderEntry,
  SettingsProviderModelRequest,
  SettingsProviderRequest,
} from "../types";

let slotKeySeed = 0;

/** Mints the editor-local row identity described on `ProviderCredentialSlotDraft`. */
function nextCredentialSlotKey(): string {
  slotKeySeed += 1;
  return `credential-slot-${slotKeySeed}`;
}

/** Auth ownership offered by the editor — mirrors `config.ProviderAuthMode`. */
const PROVIDER_AUTH_MODES: readonly ProviderAuthMode[] = ["native_cli", "bound_secret", "none"];

const DEFAULT_SLOT_KIND = "api_key";
const VAULT_REF_PREFIX = "vault:";

function isProviderAuthMode(value: string | undefined): value is ProviderAuthMode {
  return PROVIDER_AUTH_MODES.includes(value as ProviderAuthMode);
}

export function emptyProviderDraft(): ProviderDraft {
  return {
    name: "",
    command: "",
    display_name: "",
    model_default: "",
    curated_models: "",
    harness: "acp",
    runtime_provider: "",
    transport: "",
    base_url: "",
    auth_mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    auth_status_command: "",
    auth_login_command: "",
    credential_slots: [],
    credential_secret_values: [],
  };
}

export function providerDraftFromEntry(entry: SettingsProviderEntry): ProviderDraft {
  const slots = (entry.settings.credential_slots ?? []).map(slot => ({
    ...slot,
    key: nextCredentialSlotKey(),
  }));
  const authMode = entry.settings.auth_mode;
  return {
    name: entry.name,
    command: entry.settings.command ?? "",
    display_name: entry.settings.display_name ?? "",
    model_default: entry.settings.models?.default ?? "",
    curated_models: joinCuratedModels(entry.settings.models?.curated ?? []),
    harness: entry.settings.harness ?? "acp",
    runtime_provider: entry.settings.runtime_provider ?? "",
    transport: entry.settings.transport ?? "",
    base_url: entry.settings.base_url ?? "",
    auth_mode: isProviderAuthMode(authMode) ? authMode : "native_cli",
    env_policy: entry.settings.env_policy ?? "filtered",
    home_policy: entry.settings.home_policy ?? "operator",
    auth_status_command: entry.settings.auth_status_command ?? "",
    auth_login_command: entry.settings.auth_login_command ?? "",
    credential_slots: slots,
    // Presence comes from `entry.credentials`; plaintext never rides a read path.
    credential_secret_values: slots.map(() => ""),
  };
}

/**
 * Auth ownership is a security boundary, not a layout switch: only
 * `bound_secret` may carry credential slots (`internal/CLAUDE.md` § Provider
 * auth boundary), so leaving it drops every slot and its pending plaintext, and
 * entering it seeds the one slot the daemon requires.
 */
export function withProviderAuthMode(draft: ProviderDraft, mode: ProviderAuthMode): ProviderDraft {
  if (draft.auth_mode === mode) return draft;
  if (mode !== "bound_secret") {
    return { ...draft, auth_mode: mode, credential_slots: [], credential_secret_values: [] };
  }
  const slots =
    draft.credential_slots.length > 0 ? draft.credential_slots : [newCredentialSlot(0, true)];
  return {
    ...draft,
    auth_mode: mode,
    credential_slots: slots,
    credential_secret_values: slots.map((_, index) => draft.credential_secret_values[index] ?? ""),
  };
}

export function addProviderCredentialSlot(draft: ProviderDraft): ProviderDraft {
  const slots = [
    ...draft.credential_slots,
    newCredentialSlot(draft.credential_slots.length, false),
  ];
  return {
    ...draft,
    credential_slots: slots,
    credential_secret_values: slots.map((_, index) => draft.credential_secret_values[index] ?? ""),
  };
}

export function updateProviderCredentialSlot(
  draft: ProviderDraft,
  index: number,
  patch: Partial<ProviderCredentialSlotDraft>
): ProviderDraft {
  const current = draft.credential_slots[index];
  if (!current) return draft;
  const slots = [...draft.credential_slots];
  slots[index] = { ...current, ...patch };
  return { ...draft, credential_slots: slots };
}

export function removeProviderCredentialSlot(draft: ProviderDraft, index: number): ProviderDraft {
  return {
    ...draft,
    credential_slots: draft.credential_slots.filter((_, position) => position !== index),
    credential_secret_values: draft.credential_secret_values.filter(
      (_, position) => position !== index
    ),
  };
}

export function setProviderCredentialSecretValue(
  draft: ProviderDraft,
  index: number,
  value: string
): ProviderDraft {
  const values = [...draft.credential_secret_values];
  values[index] = value;
  return { ...draft, credential_secret_values: values };
}

export function providerCredentialSecretValue(draft: ProviderDraft, index: number): string {
  return draft.credential_secret_values[index] ?? "";
}

export function providerDraftToRequest(draft: ProviderDraft): SettingsProviderRequest {
  const settings: SettingsProviderRequest["settings"] = {};
  assignTrimmed(settings, "command", draft.command);
  assignTrimmed(settings, "display_name", draft.display_name);
  settings.models = {
    ...(draft.model_default.trim() ? { default: draft.model_default.trim() } : {}),
    curated: parseCuratedModels(draft.curated_models),
  };
  assignTrimmed(settings, "harness", draft.harness);
  assignTrimmed(settings, "runtime_provider", draft.runtime_provider);
  assignTrimmed(settings, "transport", draft.transport);
  assignTrimmed(settings, "base_url", draft.base_url);
  assignTrimmed(settings, "auth_mode", draft.auth_mode);
  assignTrimmed(settings, "env_policy", draft.env_policy);
  assignTrimmed(settings, "home_policy", draft.home_policy);
  assignTrimmed(settings, "auth_status_command", draft.auth_status_command);
  assignTrimmed(settings, "auth_login_command", draft.auth_login_command);

  const credentialSlots = requestCredentialSlots(draft);
  if (credentialSlots.length > 0) settings.credential_slots = credentialSlots;

  const secrets = requestSecrets(draft, credentialSlots);
  return secrets.length > 0 ? { settings, secrets } : { settings };
}

interface ProviderDraftValidationOptions {
  mode: "create" | "edit";
  existingNames: readonly string[];
}

/**
 * Submit gate. Mirrors the daemon's resolved-provider rules so a rejected
 * configuration never leaves the dialog: a name, a command on create, and — for
 * `bound_secret` — at least one complete slot, with plaintext only where a
 * `vault:` ref can store it.
 */
export function providerDraftIsValid(
  draft: ProviderDraft,
  { mode, existingNames }: ProviderDraftValidationOptions
): boolean {
  const name = draft.name.trim();
  if (!name) return false;
  if (mode === "create") {
    if (!draft.command.trim()) return false;
    if (existingNames.some(existing => existing.toLowerCase() === name.toLowerCase())) return false;
  }
  if (draft.harness === "pi_acp" && !draft.runtime_provider.trim()) return false;
  if (draft.auth_mode === "bound_secret" && draft.credential_slots.length === 0) return false;
  return draft.credential_slots.every((slot, index) => {
    if (draft.auth_mode !== "bound_secret") return true;
    if (!isCompleteSlot(slot)) return false;
    const value = providerCredentialSecretValue(draft, index).trim();
    return value.length === 0 || slot.secret_ref.trim().startsWith(VAULT_REF_PREFIX);
  });
}

/** A slot the daemon can resolve: identity, injection target, and a source ref. */
function isCompleteSlot(slot: SettingsProviderCredentialSlotRequest): boolean {
  return Boolean(slot.name.trim() && slot.target_env.trim() && slot.secret_ref.trim());
}

export function isVaultSecretRef(secretRef: string): boolean {
  return secretRef.trim().startsWith(VAULT_REF_PREFIX);
}

function newCredentialSlot(index: number, required: boolean): ProviderCredentialSlotDraft {
  return {
    key: nextCredentialSlotKey(),
    name: index === 0 ? DEFAULT_SLOT_KIND : "",
    target_env: "",
    secret_ref: "",
    kind: DEFAULT_SLOT_KIND,
    required,
  };
}

function requestCredentialSlots(draft: ProviderDraft): SettingsProviderCredentialSlotRequest[] {
  if (draft.auth_mode !== "bound_secret") return [];
  return draft.credential_slots.flatMap(slot => {
    if (!isCompleteSlot(slot)) return [];
    const kind = slot.kind?.trim();
    return [
      {
        name: slot.name.trim(),
        target_env: slot.target_env.trim(),
        secret_ref: slot.secret_ref.trim(),
        ...(kind ? { kind } : {}),
        required: slot.required,
      },
    ];
  });
}

function requestSecrets(
  draft: ProviderDraft,
  credentialSlots: SettingsProviderCredentialSlotRequest[]
): NonNullable<SettingsProviderRequest["secrets"]> {
  const byName = new Map(credentialSlots.map(slot => [slot.name, slot]));
  const secrets: NonNullable<SettingsProviderRequest["secrets"]> = [];
  draft.credential_slots.forEach((slot, index) => {
    const value = providerCredentialSecretValue(draft, index);
    const target = byName.get(slot.name.trim());
    if (!value.trim() || !target || !isVaultSecretRef(target.secret_ref)) return;
    secrets.push({
      name: target.name,
      secret_ref: target.secret_ref,
      kind: target.kind ?? DEFAULT_SLOT_KIND,
      value,
    });
  });
  return secrets;
}

function assignTrimmed<K extends keyof NonNullable<SettingsProviderRequest["settings"]>>(
  settings: NonNullable<SettingsProviderRequest["settings"]>,
  key: K,
  value: string
) {
  const trimmed = value.trim();
  if (trimmed) settings[key] = trimmed as NonNullable<SettingsProviderRequest["settings"]>[K];
}

function joinCuratedModels(models: SettingsProviderModelRequest[]): string {
  return models
    .flatMap(model => {
      const id = model.id.trim();
      return id ? [id] : [];
    })
    .join("\n");
}

function parseCuratedModels(raw: string): SettingsProviderModelRequest[] {
  const seen = new Set<string>();
  const models: SettingsProviderModelRequest[] = [];
  for (const part of raw.split(/[\n,]/u)) {
    const id = part.trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    models.push({ id });
  }
  return models;
}
