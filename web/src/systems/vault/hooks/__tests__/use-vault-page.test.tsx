// Suite: Vault route state
// Invariant: Vault list controls serialize their durable state through the route search contract.
// Boundary IN: useVaultPage setters and current TanStack Router search.
// Boundary OUT: Route validation and Vault API transport.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  putMutate: vi.fn(),
  putReset: vi.fn(),
  putState: {
    data: undefined as unknown,
    error: null as Error | null,
    isPending: false,
  },
  deleteMutate: vi.fn(),
  deleteReset: vi.fn(),
  deleteState: {
    error: null as Error | null,
    isPending: false,
  },
  secrets: [] as unknown[],
  allSecretsState: {
    error: null as Error | null,
    isFetching: false,
    isLoading: false,
    isSuccess: true,
  },
  vaultFilter: vi.fn(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mocks.navigate,
}));

vi.mock("@/systems/vault/adapters/vault-api", () => ({
  VaultApiError: class VaultApiError extends Error {},
}));

vi.mock("@/systems/vault/hooks/use-vault-actions", () => ({
  useDeleteVaultSecret: () => ({
    ...mocks.deleteState,
    mutate: mocks.deleteMutate,
    reset: mocks.deleteReset,
  }),
  usePutVaultSecret: () => ({ ...mocks.putState, mutate: mocks.putMutate, reset: mocks.putReset }),
}));

vi.mock("@/systems/vault/hooks/use-vault", () => ({
  useVaultSecrets: (filter: unknown) => {
    mocks.vaultFilter(filter);
    const isUnfiltered =
      typeof filter === "object" && filter !== null && Object.keys(filter).length === 0;
    return {
      data: mocks.secrets,
      error: isUnfiltered ? mocks.allSecretsState.error : null,
      isFetching: isUnfiltered ? mocks.allSecretsState.isFetching : false,
      isLoading: isUnfiltered ? mocks.allSecretsState.isLoading : false,
      isSuccess: isUnfiltered ? mocks.allSecretsState.isSuccess : true,
      refetch: vi.fn(),
    };
  },
}));

import type { VaultSecret } from "@/systems/vault";
import {
  normalizeVaultPrefixForNamespace,
  useVaultPage,
} from "@/systems/vault/hooks/use-vault-page";

const providerSecret = {
  created_at: "2026-07-18T12:00:00Z",
  kind: "api-key",
  namespace: "providers",
  ref: "vault:providers/openai",
  updated_at: "2026-07-18T12:00:00Z",
} as VaultSecret;

describe("useVaultPage route state", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.putMutate.mockReset();
    mocks.putReset.mockReset();
    mocks.putState.data = undefined;
    mocks.putState.error = null;
    mocks.putState.isPending = false;
    mocks.deleteMutate.mockReset();
    mocks.deleteReset.mockReset();
    mocks.deleteState.error = null;
    mocks.deleteState.isPending = false;
    mocks.secrets = [];
    mocks.allSecretsState.error = null;
    mocks.allSecretsState.isFetching = false;
    mocks.allSecretsState.isLoading = false;
    mocks.allSecretsState.isSuccess = true;
  });

  it("Should derive the API filter and write prefix, namespace, and view through route search", () => {
    const { result } = renderHook(() =>
      useVaultPage({ namespace: "providers", q: "vault:providers/", view: "cards" })
    );
    const current = {
      namespace: "providers" as const,
      q: "vault:providers/",
      view: "cards" as const,
    };

    // Two reads are issued: the filtered listing, plus an unfiltered pass used
    // only to detect ref collisions the active filter would otherwise hide.
    expect(mocks.vaultFilter).toHaveBeenCalledWith({
      namespace: "providers",
      prefix: "vault:providers/",
    });
    expect(mocks.vaultFilter).toHaveBeenCalledWith({});

    act(() => result.current.setPrefix("  vault:mcp/  "));
    expect(mocks.navigate.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      q: "vault:mcp/",
    });

    act(() => result.current.setNamespace("sessions"));
    expect(mocks.navigate.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      namespace: "sessions",
      q: undefined,
    });

    act(() => result.current.setView("rows"));
    expect(mocks.navigate.mock.lastCall?.[0].search(current)).toEqual({
      ...current,
      view: undefined,
    });
    expect(mocks.navigate.mock.lastCall?.[0].to).toBe("/vault");
  });

  it("Should reject a namespace-mismatched prefix during route normalization", () => {
    expect(normalizeVaultPrefixForNamespace("vault:providers/openai", "sessions")).toBeUndefined();
    expect(normalizeVaultPrefixForNamespace("vault:sessions/session-a/", "sessions")).toBe(
      "vault:sessions/session-a/"
    );
    expect(normalizeVaultPrefixForNamespace("vault:providers/openai", undefined)).toBe(
      "vault:providers/openai"
    );
  });

  it("Should replace the selected secret with its exact ref and kind, rejecting blank values", () => {
    mocks.secrets = [providerSecret];
    mocks.putMutate.mockImplementation((_payload, options) => {
      options.onSuccess(providerSecret);
    });
    const { result } = renderHook(() => useVaultPage());

    act(() => result.current.openInspect(providerSecret));
    act(() => result.current.setReplaceValue("   "));
    act(() => result.current.replaceSecret());
    expect(mocks.putMutate).not.toHaveBeenCalled();

    act(() => result.current.setReplaceValue("rotated-secret"));
    act(() => result.current.replaceSecret());

    expect(mocks.putMutate).toHaveBeenCalledWith(
      {
        kind: "api-key",
        ref: "vault:providers/openai",
        secret_value: "rotated-secret",
      },
      expect.objectContaining({ onSuccess: expect.any(Function) })
    );
    expect(result.current.replaceValue).toBe("");
    expect(result.current.lastAction).toEqual({
      kind: "saved",
      ref: "vault:providers/openai",
      secret: providerSecret,
    });
  });

  it("Should preserve the replacement input and expose the mutation error after failure", () => {
    mocks.secrets = [providerSecret];
    const { result, rerender } = renderHook(() => useVaultPage());

    act(() => result.current.openInspect(providerSecret));
    act(() => result.current.setReplaceValue("retry-this-value"));
    act(() => result.current.replaceSecret());
    mocks.putState.error = new Error("Vault write failed");
    rerender();

    expect(result.current.replaceValue).toBe("retry-this-value");
    expect(result.current.replaceError).toBe("Vault write failed");
  });

  it("Should not open deletion while replacement of the selected secret is pending", () => {
    mocks.secrets = [providerSecret];
    const { result, rerender } = renderHook(() => useVaultPage());

    act(() => result.current.openInspect(providerSecret));
    mocks.putState.isPending = true;
    rerender();
    act(() => result.current.openDelete(providerSecret));

    expect(result.current.deleteTarget).toEqual({ mode: "closed" });
    expect(mocks.deleteReset).not.toHaveBeenCalled();
  });

  it("Should block a write onto an existing ref until the overwrite is confirmed", () => {
    mocks.secrets = [providerSecret];
    const { result } = renderHook(() => useVaultPage());

    act(() => result.current.openCreate());
    act(() =>
      result.current.updateDraft(draft => ({
        ...draft,
        ref: "  vault:providers/openai  ",
        secretValue: "sk-live",
      }))
    );

    expect(result.current.editorRefExists).toBe(true);
    expect(result.current.editorIsValid).toBe(false);
    act(() => result.current.saveEditor());
    expect(mocks.putMutate).not.toHaveBeenCalled();

    act(() => result.current.updateDraft(draft => ({ ...draft, overwriteConfirmed: true })));
    expect(result.current.editorIsValid).toBe(true);
    act(() => result.current.saveEditor());
    expect(mocks.putMutate.mock.lastCall?.[0]).toEqual({
      ref: "vault:providers/openai",
      secret_value: "sk-live",
    });
  });

  it("Should allow a write onto an unused ref without any confirmation", () => {
    mocks.secrets = [providerSecret];
    const { result } = renderHook(() => useVaultPage());

    act(() => result.current.openCreate());
    act(() =>
      result.current.updateDraft(draft => ({
        ...draft,
        ref: "vault:providers/anthropic",
        secretValue: "sk-live",
      }))
    );

    expect(result.current.editorRefExists).toBe(false);
    expect(result.current.editorIsValid).toBe(true);
  });

  it("Should withhold a create write until the complete collision inventory resolves", () => {
    mocks.allSecretsState.isLoading = true;
    mocks.allSecretsState.isSuccess = false;
    const { result, rerender } = renderHook(() => useVaultPage({ namespace: "providers" }));

    act(() => result.current.openCreate());
    act(() =>
      result.current.updateDraft(draft => ({
        ...draft,
        ref: "vault:providers/anthropic",
        secretValue: "sk-live",
      }))
    );

    expect(result.current.editorRefExists).toBe(false);
    expect(result.current.editorIsValid).toBe(false);
    act(() => result.current.saveEditor());
    expect(mocks.putMutate).not.toHaveBeenCalled();

    mocks.allSecretsState.isLoading = false;
    mocks.allSecretsState.isSuccess = true;
    rerender();

    expect(result.current.editorIsValid).toBe(true);
  });

  it("Should retract the overwrite confirmation when the ref is pointed elsewhere", () => {
    mocks.secrets = [providerSecret];
    const { result } = renderHook(() => useVaultPage());

    act(() => result.current.openCreate());
    act(() =>
      result.current.updateDraft(draft => ({
        ...draft,
        ref: "vault:providers/openai",
        secretValue: "sk-live",
        overwriteConfirmed: true,
      }))
    );
    act(() => result.current.updateDraft(draft => ({ ...draft, ref: "vault:providers/other" })));
    act(() => result.current.updateDraft(draft => ({ ...draft, ref: "vault:providers/openai" })));

    expect(result.current.editorRefExists).toBe(true);
    expect(result.current.editorIsValid).toBe(false);
  });

  it("Should keep the create draft after a failed write", () => {
    mocks.secrets = [];
    const { result, rerender } = renderHook(() => useVaultPage());

    act(() => result.current.openCreate());
    act(() =>
      result.current.updateDraft(draft => ({
        ...draft,
        ref: "vault:providers/anthropic",
        kind: "api_key",
        secretValue: "sk-live",
      }))
    );
    act(() => result.current.saveEditor());
    mocks.putState.error = new Error("Vault write failed");
    rerender();

    expect(result.current.editor).toEqual({
      mode: "create",
      draft: {
        ref: "vault:providers/anthropic",
        kind: "api_key",
        secretValue: "sk-live",
        overwriteConfirmed: false,
      },
    });
    expect(result.current.editorError).toBe("Vault write failed");
  });
});
