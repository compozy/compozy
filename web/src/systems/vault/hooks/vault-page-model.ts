import { VaultApiError } from "../adapters/vault-api";
import type { VaultNamespaceFilter } from "../lib/vault-route-search";
import type { VaultListFilter, VaultSecret } from "../types";
import type { VaultDraft, VaultEditorState } from "./vault-page-logic";

const VAULT_REF_PREFIX = "vault:";

function emptyVaultDraft(): VaultDraft {
  return {
    ref: "",
    kind: "",
    secretValue: "",
    overwriteConfirmed: false,
  };
}

/**
 * Mirrors `vault.NormalizeRef` (trim only) and adds the `vault:` prefix when
 * the user typed a bare name, so the submitted ref keeps its wire shape.
 */
function normalizeVaultRef(ref: string): string {
  const trimmed = ref.trim();
  if (trimmed === "" || trimmed.startsWith(VAULT_REF_PREFIX)) return trimmed;
  return `${VAULT_REF_PREFIX}${trimmed}`;
}

function vaultErrorMessage(error: unknown): string | null {
  if (error instanceof VaultApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function vaultFilterFor(namespace: VaultNamespaceFilter, prefix: string): VaultListFilter {
  const filter: VaultListFilter = {};
  if (namespace !== "all") filter.namespace = namespace;
  const normalizedPrefix = prefix.trim();
  if (normalizedPrefix) filter.prefix = normalizedPrefix;
  return filter;
}

function findVaultSecret(
  inventory: readonly VaultSecret[],
  ref: string | null
): VaultSecret | null {
  if (!ref) return null;
  return inventory.find(secret => secret.ref === ref) ?? null;
}

/** Upsert body; an empty kind is omitted so the daemon keeps its default. */
function vaultPutRequest(ref: string, secretValue: string, kind: string | undefined) {
  const trimmedKind = kind?.trim();
  return { ref, secret_value: secretValue, ...(trimmedKind ? { kind: trimmedKind } : {}) };
}

interface VaultEditorCheck {
  editorRef: string;
  editorRefExists: boolean;
  editorIsValid: boolean;
}

/**
 * Create-editor validity: a ref and a value are required, and an existing ref
 * blocks save until the full inventory is fresh and the overwrite is confirmed.
 */
function checkVaultEditor(
  editor: VaultEditorState,
  inventory: readonly VaultSecret[] | undefined,
  inventoryReady: boolean
): VaultEditorCheck {
  if (editor.mode !== "create") {
    return { editorRef: "", editorRefExists: false, editorIsValid: false };
  }
  const editorRef = normalizeVaultRef(editor.draft.ref);
  const editorRefExists =
    editorRef !== "" &&
    (inventory ?? []).some(secret => normalizeVaultRef(secret.ref) === editorRef);
  const overwriteBlocked = !inventoryReady || (editorRefExists && !editor.draft.overwriteConfirmed);
  return {
    editorRef,
    editorRefExists,
    editorIsValid: editorRef !== "" && editor.draft.secretValue.trim() !== "" && !overwriteBlocked,
  };
}

interface VaultInventoryStatus {
  error: unknown;
  isError: boolean;
  isFetching: boolean;
  isSuccess: boolean;
}

/** Deep-link resolution error once the full inventory settles. */
function vaultSelectionError(
  routeRef: string | null,
  inventory: VaultInventoryStatus,
  selectedSecret: VaultSecret | null
): string | null {
  if (routeRef === null || inventory.isFetching) return null;
  if (inventory.isError) {
    return vaultErrorMessage(inventory.error) ?? "Vault secret unavailable";
  }
  if (inventory.isSuccess && selectedSecret === null) return "Vault secret not found";
  return null;
}

export {
  checkVaultEditor,
  emptyVaultDraft,
  findVaultSecret,
  normalizeVaultRef,
  vaultErrorMessage,
  vaultFilterFor,
  vaultPutRequest,
  vaultSelectionError,
};
