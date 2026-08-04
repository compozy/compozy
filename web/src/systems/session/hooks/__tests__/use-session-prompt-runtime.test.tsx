// Suite: Session prompt runtime availability
// Invariant: The composer only enables models allowed by the workspace whose provider is authenticated globally.
// Boundary IN: Session runtime hook plus its real TanStack Query projections and model-catalog mapper.
// Boundary OUT: HTTP adapters and rendered selector behavior, owned by adapter and RuntimeSelector suites.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const runtimeSelectionMutation = vi.hoisted(() => ({
  isPending: false,
  mutate: vi.fn(),
}));

vi.mock("../use-session-runtime-selection", () => ({
  useSetSessionRuntime: () => runtimeSelectionMutation,
}));

import { agentKeys, type AgentPayload } from "@/systems/agent";
import {
  modelCatalogKeys,
  type AllModelsListResponse,
  type ProviderModelPayload,
} from "@/systems/model-catalog";
import {
  providerKeys,
  type ProviderAuthStatus,
  type ProviderListResponse,
  type ProviderSummary,
} from "@/systems/providers";
import { primarySessionFixture } from "@/systems/session/mocks";
import { type WorkspaceDetailPayload, workspaceKeys } from "@/systems/workspace";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";

import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../../stores/session-prompt-runtime-store";
import { useSessionPromptRuntime } from "../use-session-prompt-runtime";

function model(providerId: string, modelId: string): ProviderModelPayload {
  return {
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    model_id: modelId,
    provider_id: providerId,
    sources: [],
    stale: false,
  } as ProviderModelPayload;
}

function provider(
  name: string,
  authState: NonNullable<ProviderAuthStatus>["state"]
): ProviderSummary {
  return {
    auth_status: {
      env_policy: "inherit",
      home_policy: "operator",
      mode: "native_cli",
      state: authState,
    },
    default: name === "codex",
    display_name: name,
    name,
  };
}

function createHarness() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { queryClient, wrapper };
}

describe("useSessionPromptRuntime", () => {
  beforeEach(() => {
    runtimeSelectionMutation.isPending = false;
    runtimeSelectionMutation.mutate.mockReset();
  });

  it("Should combine workspace scope with global provider authentication", async () => {
    const workspaceId = workspaceDetailFixture.workspace.id;
    const workspace: WorkspaceDetailPayload = {
      ...workspaceDetailFixture,
      agents: [],
      providers: [
        {
          display_name: "Codex",
          harness: "acp",
          name: "codex",
          runtime_provider: "codex",
        },
        {
          display_name: "Groq",
          harness: "pi_acp",
          name: "groq",
          runtime_provider: "groq",
        },
      ],
    };
    const globalProviders: ProviderListResponse = {
      providers: [
        provider("codex", "authenticated"),
        provider("groq", "missing_credential"),
        provider("claude", "authenticated"),
      ],
    };
    const catalog: AllModelsListResponse = {
      models: [
        model("codex", "gpt-5.6-sol"),
        model("groq", "llama-4"),
        model("claude", "claude-fable-5"),
      ],
    };
    const session = {
      ...primarySessionFixture,
      agent_name: "general",
      runtime: {
        status: "ready" as const,
        transition: "initial_bind" as const,
        effective: { provider: "codex" },
        selected: undefined,
        selection_revision: 0,
      },
      workspace_id: workspaceId,
    };
    const { queryClient, wrapper } = createHarness();
    queryClient.setQueryData(workspaceKeys.detail(workspaceId), workspace);
    queryClient.setQueryData<AgentPayload[]>(agentKeys.list(workspaceId), []);
    queryClient.setQueryData(providerKeys.lists(), globalProviders);
    queryClient.setQueryData(
      modelCatalogKeys.allModels("all", undefined, undefined, true),
      catalog
    );
    const store = sessionPromptRuntimeStoreLogic.createStore(
      sessionPromptRuntimeInput({
        agentName: session.agent_name,
        canPrompt: true,
        effectiveRuntime: session.runtime.effective,
        selectedRuntime: session.runtime.selected,
        selectionRevision: session.runtime.selection_revision,
        sessionId: session.id,
        workspaceId: session.workspace_id,
      })
    );

    const { result, unmount } = renderHook(() => useSessionPromptRuntime(store), { wrapper });

    await waitFor(() => expect(result.current.canSelectRuntime).toBe(true));
    expect(result.current.catalog.providers.map(entry => entry.id)).toEqual(["codex", "groq"]);
    expect(result.current.catalog.providers.find(entry => entry.id === "groq")?.needs_auth).toBe(
      true
    );
    expect(result.current.catalog.models.find(entry => entry.provider === "codex")?.disabled).toBe(
      false
    );
    expect(result.current.catalog.models.find(entry => entry.provider === "groq")).toMatchObject({
      disabled: true,
      disabled_reason: "Sign in",
    });
    expect(result.current.catalog.models.some(entry => entry.provider === "claude")).toBe(false);

    act(() => {
      result.current.onRuntimeChange({
        provider: "claude",
        model: "claude-fable-5",
        reasoning_effort: "max",
      });
    });
    expect(runtimeSelectionMutation.mutate).toHaveBeenCalledWith(
      {
        id: session.id,
        request: {
          expected_revision: 0,
          runtime: {
            provider: "claude",
            model: "claude-fable-5",
            reasoning_effort: "max",
          },
        },
        signal: expect.any(AbortSignal),
      },
      expect.objectContaining({
        onError: expect.any(Function),
        onSettled: expect.any(Function),
        onSuccess: expect.any(Function),
      })
    );
    const signal = runtimeSelectionMutation.mutate.mock.calls[0]?.[0].signal as AbortSignal;
    expect(signal.aborted).toBe(false);
    unmount();
    expect(signal.aborted).toBe(true);
  });
});
