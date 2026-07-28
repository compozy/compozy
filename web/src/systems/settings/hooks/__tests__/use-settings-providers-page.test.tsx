import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@tanstack/react-router", () => ({
  useMatchRoute: () => () => false,
}));

vi.mock("@/systems/settings/adapters/settings-api", () => ({
  getSettingsRestartStatus: vi.fn(),
  listSettingsProviders: vi.fn(),
  putSettingsProvider: vi.fn(),
  deleteSettingsProvider: vi.fn(),
  triggerSettingsRestart: vi.fn(),
  SettingsApiError: class SettingsApiError extends Error {
    status = 500;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

import {
  deleteSettingsProvider,
  listSettingsProviders,
  putSettingsProvider,
  SettingsApiError,
} from "@/systems/settings/adapters/settings-api";
import { initialSettingsRestartState } from "@/systems/settings/stores/settings-restart-store";
import { useSettingsRestartStore } from "@/systems/settings/stores/use-settings-restart-store";
import type { SettingsProviderCollection } from "@/systems/settings";
import { useSettingsProvidersPage } from "../use-settings-providers-page";

const claudeEntry: SettingsProviderCollection["providers"][number] = {
  name: "claude",
  default: true,
  command_available: true,
  auth_status: {
    mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    state: "authenticated",
  },
  settings: {
    command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
    models: {
      default: "claude-sonnet-5",
      curated: [{ id: "claude-sonnet-5" }, { id: "claude-haiku-4-5-20251001" }],
    },
    auth_mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    auth_status_command: "claude auth status",
    auth_login_command: "claude login",
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
    shadowed_sources: [{ kind: "builtin-provider", scope: "global" }],
  },
  fallback: {
    settings: {
      command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
      models: { default: "claude-sonnet-5" },
    },
    source: { kind: "builtin-provider", scope: "global" },
  },
};

const codexEntry: SettingsProviderCollection["providers"][number] = {
  name: "codex",
  default: false,
  command_available: true,
  auth_status: {
    mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    state: "unknown",
  },
  settings: {
    command: "npx -y @agentclientprotocol/codex-acp@latest",
    models: {
      default: "gpt-5.6-sol",
      curated: [
        {
          id: "gpt-5.6-sol",
          supports_reasoning: true,
          reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
          default_reasoning_effort: "medium",
        },
        {
          id: "gpt-5.6-terra",
          supports_reasoning: true,
          reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
          default_reasoning_effort: "medium",
        },
        {
          id: "gpt-5.6-luna",
          supports_reasoning: true,
          reasoning_efforts: ["none", "low", "medium", "high", "xhigh", "max"],
          default_reasoning_effort: "medium",
        },
      ],
      reasoning: { apply: "acp_option" },
    },
    auth_mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    auth_status_command: "codex auth status",
    auth_login_command: "codex login",
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "builtin-provider", scope: "global" },
  },
};

const collection: SettingsProviderCollection = {
  collection: "providers",
  scope: "global",
  available_scopes: ["global"],
  providers: [claudeEntry, codexEntry],
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  useSettingsRestartStore.setState({
    ...initialSettingsRestartState,
    startRestart: useSettingsRestartStore.getState().startRestart,
    updateRestart: useSettingsRestartStore.getState().updateRestart,
    clearRestart: useSettingsRestartStore.getState().clearRestart,
    recordMutation: useSettingsRestartStore.getState().recordMutation,
  });
  vi.mocked(listSettingsProviders).mockResolvedValue(collection);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSettingsProvidersPage", () => {
  it("exposes the provider collection and installation counts", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => {
      expect(result.current.providers).toHaveLength(2);
    });

    expect(result.current.counts).toEqual({
      total: 2,
      installed: 1,
      binaryMissing: 0,
      needsSetup: 1,
    });
  });

  it("opens create editor with an empty draft", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
    });

    expect(result.current.inspector).toMatchObject({
      mode: "create",
      draft: {
        name: "",
        command: "",
        model_default: "",
        curated_models: "",
        credential_slots: [],
        auth_mode: "native_cli",
      },
    });
  });

  it("blocks create save when the name collides with an existing provider", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({ ...draft, name: "Claude" }));
    });

    expect(result.current.inspectorIsValid).toBe(false);
  });

  it("opens edit editor seeded from the entry and keeps the selected item", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openInspect(claudeEntry);
      result.current.switchToEdit();
    });

    expect(result.current.inspector).toMatchObject({
      mode: "edit",
      entry: expect.objectContaining({ name: "claude" }),
      draft: expect.objectContaining({
        command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
        model_default: "claude-sonnet-5",
        curated_models: "claude-sonnet-5\nclaude-haiku-4-5-20251001",
        credential_slots: [],
        auth_mode: "native_cli",
        env_policy: "filtered",
        home_policy: "operator",
        auth_status_command: "claude auth status",
        auth_login_command: "claude login",
      }),
    });
  });

  it("submits a full replacement on save and records the last action", async () => {
    vi.mocked(putSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openInspect(claudeEntry);
      result.current.switchToEdit();
    });
    act(() => {
      result.current.updateDraft(draft => ({
        ...draft,
        model_default: "claude-haiku",
        curated_models: "claude-haiku\nclaude-sonnet-5",
      }));
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("saved");
    });

    expect(putSettingsProvider).toHaveBeenCalledWith("claude", {
      settings: {
        command: "npx -y @agentclientprotocol/claude-agent-acp@latest",
        models: {
          default: "claude-haiku",
          curated: [{ id: "claude-haiku" }, { id: "claude-sonnet-5" }],
        },
        harness: "acp",
        auth_mode: "native_cli",
        env_policy: "filtered",
        home_policy: "operator",
        auth_status_command: "claude auth status",
        auth_login_command: "claude login",
      },
    });
    expect(result.current.inspector.mode).toBe("closed");
  });

  it("preserves additional credential slots when editing provider metadata", async () => {
    vi.mocked(putSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });
    const providerWithMultipleCredentials: SettingsProviderCollection["providers"][number] = {
      ...claudeEntry,
      name: "openrouter",
      settings: {
        command: "npx -y pi-acp@latest",
        models: {
          default: "openai/gpt-5.6-sol",
          curated: [{ id: "openai/gpt-5.6-sol", supports_reasoning: true }],
        },
        harness: "pi_acp",
        runtime_provider: "openrouter",
        auth_mode: "bound_secret",
        env_policy: "filtered",
        home_policy: "operator",
        credential_slots: [
          {
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: false,
          },
          {
            name: "organization",
            target_env: "OPENROUTER_ORG_ID",
            secret_ref: "env:OPENROUTER_ORG_ID",
            kind: "organization",
            required: true,
          },
        ],
      },
      credentials: [
        {
          name: "api_key",
          target_env: "OPENROUTER_API_KEY",
          secret_ref: "vault:providers/openrouter/api-key",
          present: true,
          required: false,
        },
        {
          name: "organization",
          target_env: "OPENROUTER_ORG_ID",
          secret_ref: "env:OPENROUTER_ORG_ID",
          present: true,
          required: true,
        },
      ],
    };
    vi.mocked(listSettingsProviders).mockResolvedValue({
      ...collection,
      providers: [providerWithMultipleCredentials],
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(1));

    act(() => {
      result.current.openInspect(providerWithMultipleCredentials);
      result.current.switchToEdit();
    });
    act(() => {
      result.current.updateDraft(draft => ({
        ...draft,
        model_default: "anthropic/claude-sonnet",
        credential_slots: draft.credential_slots.map((slot, index) =>
          index === 1 ? { ...slot, secret_ref: "vault:providers/openrouter/organization" } : slot
        ),
        credential_secret_values: ["", "org-secret"],
      }));
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("saved");
    });

    expect(putSettingsProvider).toHaveBeenCalledWith("openrouter", {
      settings: {
        command: "npx -y pi-acp@latest",
        models: {
          default: "anthropic/claude-sonnet",
          curated: [{ id: "openai/gpt-5.6-sol" }],
        },
        harness: "pi_acp",
        runtime_provider: "openrouter",
        auth_mode: "bound_secret",
        env_policy: "filtered",
        home_policy: "operator",
        credential_slots: [
          {
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: false,
          },
          {
            name: "organization",
            target_env: "OPENROUTER_ORG_ID",
            secret_ref: "vault:providers/openrouter/organization",
            kind: "organization",
            required: true,
          },
        ],
      },
      secrets: [
        {
          name: "organization",
          secret_ref: "vault:providers/openrouter/organization",
          kind: "organization",
          value: "org-secret",
        },
      ],
    });
  });

  it("submits vault-backed provider secrets without reading them back", async () => {
    vi.mocked(putSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({
        ...draft,
        name: "openrouter",
        command: "npx -y pi-acp@latest",
        model_default: "openai/gpt-5.6-sol",
        harness: "pi_acp",
        runtime_provider: "openrouter",
        auth_mode: "bound_secret",
        credential_slots: [
          {
            key: "slot-api-key",
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: true,
          },
        ],
        credential_secret_values: ["sk-live"],
      }));
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("saved");
    });

    expect(putSettingsProvider).toHaveBeenCalledWith("openrouter", {
      settings: {
        command: "npx -y pi-acp@latest",
        models: {
          default: "openai/gpt-5.6-sol",
          curated: [],
        },
        harness: "pi_acp",
        runtime_provider: "openrouter",
        auth_mode: "bound_secret",
        env_policy: "filtered",
        home_policy: "operator",
        credential_slots: [
          {
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: true,
          },
        ],
      },
      secrets: [
        {
          name: "api_key",
          secret_ref: "vault:providers/openrouter/api-key",
          kind: "api_key",
          value: "sk-live",
        },
      ],
    });
  });

  it("Should reject an additional secret value without a vault-backed reference", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({
        ...draft,
        name: "openrouter",
        command: "npx -y pi-acp@latest",
        auth_mode: "bound_secret",
        credential_slots: [
          {
            key: "slot-api-key",
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: true,
          },
          {
            key: "slot-organization",
            name: "organization",
            target_env: "OPENROUTER_ORG_ID",
            secret_ref: "env:OPENROUTER_ORG_ID",
            kind: "organization",
            required: false,
          },
        ],
        credential_secret_values: ["", "org-secret"],
      }));
    });

    expect(result.current.inspectorIsValid).toBe(false);

    act(() => {
      result.current.saveInspector();
    });
    expect(putSettingsProvider).not.toHaveBeenCalled();
  });

  it("Should drop credential slots from the request when auth ownership is not bound_secret", async () => {
    vi.mocked(putSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({
        ...draft,
        name: "local-acp",
        command: "npx -y local-acp@latest",
        auth_mode: "none",
        // Slots left over from an earlier bound_secret draft must never reach the
        // daemon: it rejects credential_slots under any other auth mode.
        credential_slots: [
          {
            key: "slot-api-key",
            name: "api_key",
            target_env: "LOCAL_API_KEY",
            secret_ref: "vault:providers/local-acp/api-key",
            kind: "api_key",
            required: true,
          },
        ],
        credential_secret_values: ["sk-live"],
      }));
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("saved");
    });

    const [, body] = vi.mocked(putSettingsProvider).mock.calls.at(-1)!;
    expect(body.settings?.credential_slots).toBeUndefined();
    expect(body.secrets).toBeUndefined();
    expect(body.settings?.auth_mode).toBe("none");
  });

  it("Should block create save until the provider carries a command", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({ ...draft, name: "openrouter" }));
    });
    expect(result.current.inspectorIsValid).toBe(false);

    act(() => {
      result.current.updateDraft(draft => ({ ...draft, command: "npx -y pi-acp@latest" }));
    });
    expect(result.current.inspectorIsValid).toBe(true);
  });

  it("Should block pi_acp saves until the runtime provider is set", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openCreate();
      result.current.updateDraft(draft => ({
        ...draft,
        name: "openrouter",
        command: "npx -y pi-acp@latest",
        harness: "pi_acp",
      }));
    });
    expect(result.current.inspectorIsValid).toBe(false);

    act(() => {
      result.current.updateDraft(draft => ({ ...draft, runtime_provider: "openrouter" }));
    });
    expect(result.current.inspectorIsValid).toBe(true);
  });

  it("Should seed an edit draft with empty secret values for every stored slot", async () => {
    const boundProvider: SettingsProviderCollection["providers"][number] = {
      ...claudeEntry,
      name: "openrouter",
      settings: {
        ...claudeEntry.settings,
        auth_mode: "bound_secret",
        credential_slots: [
          {
            name: "api_key",
            target_env: "OPENROUTER_API_KEY",
            secret_ref: "vault:providers/openrouter/api-key",
            kind: "api_key",
            required: true,
          },
        ],
      },
      credentials: [
        {
          name: "api_key",
          target_env: "OPENROUTER_API_KEY",
          secret_ref: "vault:providers/openrouter/api-key",
          present: true,
          required: true,
        },
      ],
    };
    vi.mocked(listSettingsProviders).mockResolvedValue({
      ...collection,
      providers: [boundProvider],
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(1));

    act(() => {
      result.current.openInspect(boundProvider);
      result.current.switchToEdit();
    });

    const inspector = result.current.inspector;
    expect(inspector.mode).toBe("edit");
    if (inspector.mode !== "edit") return;
    // Presence rides `credentials[]`; no read path returns the value itself.
    expect(inspector.draft.credential_secret_values).toEqual([""]);
    expect(inspector.draft.credential_slots).toHaveLength(1);
  });

  it("Should send membership ids without copying merged catalog metadata", async () => {
    vi.mocked(putSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: false,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openInspect(codexEntry);
      result.current.switchToEdit();
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("saved");
    });

    expect(putSettingsProvider).toHaveBeenCalledWith("codex", {
      settings: {
        command: "npx -y @agentclientprotocol/codex-acp@latest",
        models: {
          default: "gpt-5.6-sol",
          curated: [{ id: "gpt-5.6-sol" }, { id: "gpt-5.6-terra" }, { id: "gpt-5.6-luna" }],
        },
        harness: "acp",
        auth_mode: "native_cli",
        env_policy: "filtered",
        home_policy: "operator",
        auth_status_command: "codex auth status",
        auth_login_command: "codex login",
      },
    });
  });

  it("surfaces validation errors from the adapter without closing the editor", async () => {
    vi.mocked(putSettingsProvider).mockRejectedValue(
      new SettingsApiError("invalid credential_slots[0].secret_ref", 400)
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openInspect(claudeEntry);
      result.current.switchToEdit();
    });
    act(() => {
      result.current.saveInspector();
    });

    await waitFor(() => {
      expect(result.current.inspectorError).toBe("invalid credential_slots[0].secret_ref");
    });
    expect(result.current.inspector.mode).toBe("edit");
    expect(result.current.lastAction).toBeNull();
  });

  it("marks a delete action with fallback metadata for overlaid providers", async () => {
    vi.mocked(deleteSettingsProvider).mockResolvedValue({
      section: "general",
      scope: "global",
      applied: true,
      active_config_hash: "sha256:test-active",
      active_generation: 1,
      apply_record_id: "cfg_apply_test",
      lifecycle: "live",
      next_action: "none",
      restart_required: true,
      write_target: "global-config",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openDelete(claudeEntry);
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.lastAction?.kind).toBe("deleted");
    });

    expect(deleteSettingsProvider).toHaveBeenCalledWith("claude");
    expect(result.current.lastAction).toMatchObject({
      name: "claude",
      hadFallback: true,
    });
  });

  it("surfaces conflict errors from delete without mutating selection", async () => {
    vi.mocked(deleteSettingsProvider).mockRejectedValue(
      new SettingsApiError("provider is in use", 409)
    );

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSettingsProvidersPage(), { wrapper });

    await waitFor(() => expect(result.current.providers).toHaveLength(2));

    act(() => {
      result.current.openDelete(claudeEntry);
    });
    act(() => {
      result.current.confirmDelete();
    });

    await waitFor(() => {
      expect(result.current.deleteError).toBe("provider is in use");
    });
    expect(result.current.deleteTarget.mode).toBe("open");
    expect(result.current.lastAction).toBeNull();
  });
});
