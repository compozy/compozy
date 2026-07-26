import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import { VaultApiError } from "../adapters/vault-api";
import { useDeleteVaultSecret, usePutVaultSecret } from "./use-vault-actions";
import { useVaultSecrets } from "./use-vault";
import {
  VAULT_NAMESPACES,
  type VaultListFilter,
  type VaultNamespace,
  type VaultSecret,
} from "../types";

export type VaultNamespaceFilter = VaultNamespace | "all";

export interface VaultRouteSearch {
  q?: string;
  namespace?: VaultNamespace;
  view?: ListingViewMode;
}

export interface VaultDraft {
  ref: string;
  kind: string;
  secretValue: string;
  /**
   * `PUT /api/vault/secrets` is an upsert, so writing an existing ref rotates it.
   * The operator must affirm that before the write is allowed.
   */
  overwriteConfirmed: boolean;
}

export type VaultEditorState = { mode: "closed" } | { mode: "create"; draft: VaultDraft };
export type VaultDeleteState = { mode: "closed" } | { mode: "open"; secret: VaultSecret };
export type VaultLastAction =
  | { kind: "saved"; ref: string; secret: VaultSecret }
  | { kind: "deleted"; ref: string };

function emptyDraft(): VaultDraft {
  return {
    ref: "vault:sessions/",
    kind: "",
    secretValue: "",
    overwriteConfirmed: false,
  };
}

/** Mirrors `vault.NormalizeRef` — the daemon compares refs after trimming only. */
export function normalizeVaultRef(ref: string): string {
  return ref.trim();
}

function errorMessage(error: unknown): string | null {
  if (error instanceof VaultApiError) return error.message;
  if (error instanceof Error) return error.message;
  return null;
}

function normalizePrefix(value: string): string {
  return value.trim();
}

export function normalizeVaultPrefixForNamespace(
  value: unknown,
  namespace: VaultNamespace | undefined
): string | undefined {
  const prefix = normalizeListingSearchValue(value);
  if (!prefix || !namespace) return prefix;

  const match = /^vault:([^/]+)\//.exec(prefix);
  return match && match[1] !== namespace ? undefined : prefix;
}

function filterFor(namespace: VaultNamespaceFilter, prefix: string): VaultListFilter {
  const filter: VaultListFilter = {};
  if (namespace !== "all") {
    filter.namespace = namespace;
  }
  const normalizedPrefix = normalizePrefix(prefix);
  if (normalizedPrefix) {
    filter.prefix = normalizedPrefix;
  }
  return filter;
}

function countVaultSecrets(secrets: VaultSecret[]) {
  const byNamespace = Object.fromEntries(VAULT_NAMESPACES.map(item => [item, 0])) as Record<
    VaultNamespace,
    number
  >;
  for (const secret of secrets) {
    if (secret.namespace in byNamespace) {
      byNamespace[secret.namespace as VaultNamespace] += 1;
    }
  }
  return {
    total: secrets.length,
    sessions: byNamespace.sessions,
    providers: byNamespace.providers,
    byNamespace,
  };
}

export function parseVaultNamespaceFilter(value: unknown): VaultNamespace | undefined {
  if (typeof value !== "string") return undefined;
  return (VAULT_NAMESPACES as readonly string[]).includes(value)
    ? (value as VaultNamespace)
    : undefined;
}

export function validateVaultSearch(search: Record<string, unknown>): VaultRouteSearch {
  const namespace = parseVaultNamespaceFilter(search.namespace);
  return {
    q: normalizeVaultPrefixForNamespace(search.q, namespace),
    namespace,
    view: parseListingView(search.view),
  };
}

export function useVaultPage(search: VaultRouteSearch = {}) {
  const navigate = useNavigate({ from: "/vault" });
  const prefix = search.q ?? "";
  const namespace: VaultNamespaceFilter = search.namespace ?? "all";
  const view: ListingViewMode = search.view ?? "rows";

  const [editor, setEditor] = useState<VaultEditorState>({ mode: "closed" });
  const [deleteTarget, setDeleteTarget] = useState<VaultDeleteState>({ mode: "closed" });
  const [lastAction, setLastAction] = useState<VaultLastAction | null>(null);
  const [selectedRef, setSelectedRef] = useState<string | null>(null);
  const [replaceValue, setReplaceValue] = useState("");

  const filter = filterFor(namespace, prefix);
  const query = useVaultSecrets(filter);
  // Collision detection must see every ref, not just the filtered listing — an
  // active namespace/prefix filter would otherwise hide the ref being overwritten.
  const allSecretsQuery = useVaultSecrets({});
  const putMutation = usePutVaultSecret();
  const deleteMutation = useDeleteVaultSecret();

  const secrets = query.data ?? [];
  const counts = countVaultSecrets(secrets);
  const selectedSecret = selectedRef
    ? (secrets.find(secret => secret.ref === selectedRef) ?? null)
    : null;

  const updateSearch = (updater: (current: VaultRouteSearch) => VaultRouteSearch) => {
    void navigate({
      search: current => updater((current as VaultRouteSearch | undefined) ?? {}),
      to: "/vault",
    });
  };

  const setPrefix = (nextPrefix: string) => {
    updateSearch(current => ({
      ...current,
      q: normalizeListingSearchValue(nextPrefix),
    }));
  };

  const setNamespace = (next: VaultNamespaceFilter) => {
    updateSearch(current => ({
      ...current,
      q: normalizeVaultPrefixForNamespace(current.q, next === "all" ? undefined : next),
      namespace: next === "all" ? undefined : next,
    }));
  };

  const setView = (nextView: ListingViewMode) => {
    updateSearch(current => ({
      ...current,
      view: nextView === "rows" ? undefined : nextView,
    }));
  };

  const openCreate = () => {
    putMutation.reset();
    setSelectedRef(null);
    setReplaceValue("");
    setEditor({ mode: "create", draft: emptyDraft() });
  };

  const closeEditor = () => {
    setEditor({ mode: "closed" });
    putMutation.reset();
  };

  const updateDraft = (updater: (draft: VaultDraft) => VaultDraft) => {
    setEditor(current => {
      if (current.mode === "closed") return current;
      const nextDraft = updater(current.draft);
      // Pointing at a different ref retracts the overwrite affirmation — the
      // operator confirmed replacing one specific secret, not any secret.
      const refChanged = normalizeVaultRef(nextDraft.ref) !== normalizeVaultRef(current.draft.ref);
      return {
        ...current,
        draft: refChanged ? { ...nextDraft, overwriteConfirmed: false } : nextDraft,
      };
    });
  };

  const collisionInventoryReady = allSecretsQuery.isSuccess;
  const knownRefs = collisionInventoryReady ? allSecretsQuery.data : [];
  const editorRef = editor.mode === "closed" ? "" : normalizeVaultRef(editor.draft.ref);
  const editorRefExists =
    editorRef !== "" && knownRefs.some(secret => normalizeVaultRef(secret.ref) === editorRef);
  const overwriteBlocked =
    editor.mode !== "closed" &&
    (!collisionInventoryReady || (editorRefExists && !editor.draft.overwriteConfirmed));

  const editorIsValid =
    editor.mode !== "closed" &&
    editor.draft.ref.trim().startsWith("vault:") &&
    editor.draft.secretValue.trim() !== "" &&
    !overwriteBlocked;

  const saveEditor = () => {
    if (editor.mode === "closed" || !editorIsValid) return;
    const ref = editor.draft.ref.trim();
    const kind = editor.draft.kind.trim();
    putMutation.mutate(
      {
        ref,
        secret_value: editor.draft.secretValue,
        ...(kind ? { kind } : {}),
      },
      {
        onSuccess: secret => {
          setLastAction({ kind: "saved", ref, secret });
          setEditor({ mode: "closed" });
        },
      }
    );
  };

  const openInspect = (secret: VaultSecret) => {
    putMutation.reset();
    setEditor({ mode: "closed" });
    setReplaceValue("");
    setSelectedRef(secret.ref);
  };

  const closeInspect = () => {
    setSelectedRef(null);
    setReplaceValue("");
    if (editor.mode === "closed") {
      putMutation.reset();
    }
  };

  const replaceIsValid = Boolean(
    selectedSecret &&
    replaceValue.trim() !== "" &&
    deleteTarget.mode === "closed" &&
    !deleteMutation.isPending
  );

  const replaceSecret = () => {
    if (!selectedSecret || !replaceIsValid || deleteMutation.isPending) return;
    const kind = selectedSecret.kind?.trim();
    putMutation.mutate(
      {
        ref: selectedSecret.ref,
        secret_value: replaceValue,
        ...(kind ? { kind } : {}),
      },
      {
        onSuccess: secret => {
          setLastAction({ kind: "saved", ref: secret.ref, secret });
          setReplaceValue("");
        },
      }
    );
  };

  const openDelete = (secret: VaultSecret) => {
    if (putMutation.isPending) return;
    deleteMutation.reset();
    setDeleteTarget({ mode: "open", secret });
  };

  const closeDelete = () => {
    setDeleteTarget({ mode: "closed" });
    deleteMutation.reset();
  };

  const confirmDelete = () => {
    if (deleteTarget.mode !== "open" || putMutation.isPending || deleteMutation.isPending) return;
    const ref = deleteTarget.secret.ref;
    deleteMutation.mutate(ref, {
      onSuccess: () => {
        setLastAction({ kind: "deleted", ref });
        setDeleteTarget({ mode: "closed" });
        if (selectedRef === ref) {
          setSelectedRef(null);
          setReplaceValue("");
        }
      },
    });
  };

  const dismissLastAction = () => {
    setLastAction(null);
  };

  const putError = errorMessage(putMutation.error);

  return {
    counts,
    deleteError: errorMessage(deleteMutation.error),
    deleteIsPending: deleteMutation.isPending,
    deleteTarget,
    dismissLastAction,
    editor,
    editorError:
      editor.mode === "create" ? (putError ?? errorMessage(allSecretsQuery.error)) : null,
    editorIsSaving: editor.mode === "create" && putMutation.isPending,
    editorIsValid,
    editorRefExists,
    filter,
    isLoading: query.isLoading,
    isRefetching: query.isFetching && !query.isLoading,
    lastAction,
    namespace,
    prefix,
    queryError: errorMessage(query.error),
    refetch: query.refetch,
    replaceError: selectedSecret ? putError : null,
    replaceIsPending: Boolean(selectedSecret) && putMutation.isPending && editor.mode === "closed",
    replaceIsValid,
    replaceSecret,
    replaceValue,
    secrets,
    selectedSecret,
    setNamespace,
    setPrefix,
    setReplaceValue,
    setView,
    closeDelete,
    closeEditor,
    closeInspect,
    confirmDelete,
    openCreate,
    openDelete,
    openInspect,
    saveEditor,
    updateDraft,
    view,
  };
}
