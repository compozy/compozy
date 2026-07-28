// Suite: Agent runtime source availability
// Invariant: The runtime selector explains and recovers from failures in its active provider source.
// Boundary IN: Agent runtime controller composition and provider-query selection.
// Boundary OUT: Provider transport behavior, owned by the settings and workspace query suites.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

import { AgentRuntimeControl } from "../agent-runtime-control";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  settings: {
    data: undefined as { providers: [] } | undefined,
    error: null as Error | null,
    isFetching: false,
    isLoading: false,
    refetch: vi.fn(),
  },
  workspace: {
    data: undefined as { providers: [] } | undefined,
    error: null as Error | null,
    isLoading: false,
    refetch: vi.fn(),
  },
}));

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: () => ({ invalidateQueries: vi.fn(), refetchQueries: vi.fn() }),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mocks.navigate,
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));

vi.mock("@/systems/model-catalog", () => ({
  providerNeedsAuth: () => false,
  useRuntimeModelCatalog: () => ({
    error: null,
    loaded: true,
    loading: false,
    models: [],
    refresh: vi.fn(),
    refreshing: false,
  }),
}));

vi.mock("@/systems/runtime", () => ({
  RuntimeSelector: ({
    disabled,
    value,
  }: {
    disabled?: boolean;
    value: { provider: string; model: string; reasoning_effort: string };
  }) => (
    <button
      disabled={disabled}
      type="button"
      data-provider={value.provider}
      data-model={value.model}
      data-reasoning-effort={value.reasoning_effort}
    >
      Runtime selector
    </button>
  ),
}));

vi.mock("@/systems/settings", () => ({
  useSettingsProviders: () => mocks.settings,
}));

vi.mock("@/systems/workspace", () => ({
  useWorkspace: () => mocks.workspace,
}));

vi.mock("../../hooks/use-agents", () => ({
  useUpdateAgent: () => ({ isPending: false, mutate: vi.fn() }),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.settings.data = undefined;
  mocks.settings.error = null;
  mocks.settings.isFetching = false;
  mocks.settings.isLoading = false;
  mocks.workspace.data = undefined;
  mocks.workspace.error = null;
  mocks.workspace.isLoading = false;
});

describe("AgentRuntimeControl provider sources", () => {
  it("Should display the effective runtime for an agent with blank authored fields", () => {
    mocks.workspace.data = { providers: [] };

    render(
      <AgentRuntimeControl
        agent={{
          ...primaryAgentFixture,
          provider: "",
          model: undefined,
          reasoning_effort: undefined,
          effective_runtime: {
            provider: "codex",
            model: "gpt-5.4",
            reasoning_effort: "high",
            sources: {
              provider: "project_default",
              model: "provider_default",
              reasoning_effort: "model_default",
            },
          },
        }}
        workspaceId="workspace-a"
      />
    );

    const selector = screen.getByRole("button", { name: "Runtime selector" });
    expect(selector).toHaveAttribute("data-provider", "codex");
    expect(selector).toHaveAttribute("data-model", "gpt-5.4");
    expect(selector).toHaveAttribute("data-reasoning-effort", "high");
  });

  it("Should surface and retry a global provider-source failure", async () => {
    const user = userEvent.setup();
    mocks.settings.error = new Error("Global providers unavailable");

    render(
      <AgentRuntimeControl
        agent={{ ...primaryAgentFixture, origin: "global" }}
        workspaceId={null}
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Global providers unavailable");
    expect(screen.getByRole("button", { name: "Runtime selector" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Retry providers" }));
    expect(mocks.settings.refetch).toHaveBeenCalledTimes(1);
    expect(mocks.workspace.refetch).not.toHaveBeenCalled();
  });

  it("Should surface and retry a workspace provider-source failure", async () => {
    const user = userEvent.setup();
    mocks.workspace.error = new Error("Workspace providers unavailable");

    render(
      <AgentRuntimeControl
        agent={{ ...primaryAgentFixture, origin: "workspace" }}
        workspaceId="workspace-a"
      />
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Workspace providers unavailable");
    await user.click(screen.getByRole("button", { name: "Retry providers" }));
    expect(mocks.workspace.refetch).toHaveBeenCalledTimes(1);
    expect(mocks.settings.refetch).not.toHaveBeenCalled();
  });
});
