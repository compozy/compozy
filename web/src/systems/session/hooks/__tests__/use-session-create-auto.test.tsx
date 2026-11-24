// Suite: Session create Auto placement and local execution off (US-014, US-015, UT-106)
// Invariant: Selecting Auto previews placement via POST /api/computers/placement, displays
//            "Auto → <computer>", posts computer="auto" and navigates to the server-placed
//            computer_id; when local execution is off and no computer is eligible, the draft
//            enters no_eligible_computer and Send is disabled; This computer is omitted when
//            local_run_agents is false.
// Boundary IN: SessionCreateDialogHost, useSessionCreateDialogViewModel, useComputerPlacement.
// Boundary OUT: Backend placement scoring and daemon session creation (Go UT-100..UT-104, IT-090..IT-091).

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TooltipProvider } from "@compozy/ui";
import type { AgentPayload } from "@/systems/agent";
import { agentFixtures } from "@/systems/agent/mocks";
import * as computersApi from "@/systems/computers/adapters/computers-api";
import type { ComputerPayload } from "@/systems/computers";
import { primarySessionFixture } from "@/systems/session/mocks/fixtures";
import * as workspaceApi from "@/systems/workspace/adapters/workspace-api";
import type { WorkspacePayload } from "@/systems/workspace";

import { SessionCreateDialogHost } from "../../components/session-create-dialog-host";
import { createSessionCreateStore } from "../../stores/session-create-store";

const { mockMutateAsync, mockNavigate, mockUseAgents, mockActive } = vi.hoisted(() => ({
  mockMutateAsync: vi.fn(),
  mockNavigate: vi.fn(),
  mockUseAgents: vi.fn(),
  mockActive: {
    current: {
      breadthMode: "focused" as "focused" | "all",
      pending: false,
      projectWorkspaces: [] as WorkspacePayload[],
      dataScope: { kind: "project", workspaceId: "ws_local_api" } as const,
    },
  },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mockNavigate }));

vi.mock("@/systems/agent/hooks/use-agents", () => ({
  useAgents: (workspaceId: string, options?: unknown) => mockUseAgents(workspaceId, options),
}));

vi.mock("@/systems/workspace", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/workspace")>();
  return {
    ...actual,
    useActiveWorkspace: () => mockActive.current,
    useUserHomeDir: () => "/Users/pedro",
  };
});

vi.mock("@/systems/workspace/hooks/use-worktrees", () => ({
  useWorktrees: () => ({ data: undefined }),
}));

vi.mock("@/systems/workspace/hooks/use-worktree-materialization", () => ({
  useWorktreeMaterialization: () => ({
    status: "idle",
    worktree: undefined,
    cancel: vi.fn(),
    recover: vi.fn(),
    reset: vi.fn(),
    retry: vi.fn(),
    start: vi.fn(),
  }),
}));

vi.mock("@/systems/profiles", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/profiles")>();
  return {
    ...actual,
    useProfileReadScope: () => ({ aggregate: false, destination: "default" }),
    useProfiles: () => ({ data: [] }),
  };
});

vi.mock("../use-session-actions", () => ({
  useCreateSession: () => ({ mutateAsync: mockMutateAsync, isPending: false }),
}));

const WORKSPACE_API: WorkspacePayload = {
  id: "ws_local_api",
  name: "api",
  root_dir: "/Users/pedro/Dev/api",
  add_dirs: [],
  default_agent: "general",
  created_at: "2026-11-01T00:00:00Z",
  updated_at: "2026-11-01T00:00:00Z",
};

const GENERAL_AGENT: AgentPayload = {
  ...agentFixtures[0]!,
  name: "general",
  label: "General",
};

const STUDIO_COMPUTER: ComputerPayload = {
  id: "cmp_studio",
  account_id: "acc_1",
  name: "studio",
  enabled: true,
  status: "connected",
  plan_state: "ok",
  version: "0.8.0",
  platform: "darwin_arm64",
  route_in_use: { id: "rt_1", kind: "direct", label: "via LAN" },
  routes: [],
  scopes: ["sessions:operate"],
  diagnosis: null,
  protocol: { min: 1, max: 1 },
  load_balancing_weight: 100,
  added_at: "2026-11-01T00:00:00Z",
  last_connected_at: "2026-11-24T12:00:00Z",
};

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>{children}</TooltipProvider>
    </QueryClientProvider>
  );
}

describe("SessionCreateDialog Auto placement & local execution off (US-014, US-015, UT-106)", () => {
  beforeEach(() => {
    mockMutateAsync.mockReset();
    mockNavigate.mockReset();
    mockUseAgents.mockReturnValue({ data: [GENERAL_AGENT] });
    vi.spyOn(workspaceApi, "fetchWorkspaces").mockResolvedValue([WORKSPACE_API]);
    mockActive.current = {
      breadthMode: "focused",
      pending: false,
      projectWorkspaces: [WORKSPACE_API],
      dataScope: { kind: "project", workspaceId: WORKSPACE_API.id },
    };
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("US-015: resolves Auto → studio from placement preview, submits computer='auto', and navigates to placed computer_id", async () => {
    const user = userEvent.setup();
    vi.spyOn(computersApi, "fetchComputers").mockResolvedValue({
      local: { run_agents: true, load_balancing: true, load_balancing_weight: 50 },
      computers: [STUDIO_COMPUTER],
    });
    vi.spyOn(computersApi, "fetchHostResources").mockResolvedValue({
      cpu_count: 12,
      cpu_util: 0.2,
      mem_total: 32_000_000_000,
      mem_available: 20_000_000_000,
      sampled_at: new Date().toISOString(),
    });
    const previewSpy = vi.spyOn(computersApi, "previewComputerPlacement").mockResolvedValue({
      computer_id: "cmp_studio",
      name: "studio",
      workspace_id: "ws_studio_api",
    });
    mockMutateAsync.mockResolvedValue({
      ...primarySessionFixture,
      id: "sess-auto-1",
      agent_name: "general",
      workspace_id: "ws_studio_api",
      computer_id: "cmp_studio",
    });

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "general", workspaceId: WORKSPACE_API.id });

    render(
      <SessionCreateDialogHost
        activeWorkspace={WORKSPACE_API}
        agents={[GENERAL_AGENT]}
        dataScope={{ kind: "project", workspaceId: WORKSPACE_API.id }}
        projectWorkspaceId={WORKSPACE_API.id}
        store={store}
      />,
      { wrapper: createWrapper(queryClient) }
    );

    const picker = await screen.findByTestId("session-create-run-on");
    await user.click(picker);
    await user.click(screen.getByTestId("session-create-run-on-item-auto"));

    await waitFor(() => {
      expect(previewSpy).toHaveBeenCalledWith(
        { workspace: WORKSPACE_API.id },
        expect.any(AbortSignal)
      );
      expect(screen.getByTestId("session-create-run-on")).toHaveTextContent("Auto");
      expect(screen.getByTestId("session-create-run-on")).toHaveTextContent("→ studio");
      expect(screen.getByTestId("auto-resolution-panel")).toBeInTheDocument();
    });

    await user.click(screen.getByTestId("session-create-submit"));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          agent_name: "general",
          workspace: WORKSPACE_API.id,
          computer: "auto",
        })
      );
      expect(mockNavigate).toHaveBeenCalledWith(
        expect.objectContaining({
          to: "/agents/$name/sessions/$id",
          params: { name: "general", id: "sess-auto-1" },
          search: expect.objectContaining({ computer: "cmp_studio" }),
        })
      );
    });
  });

  it("UT-106: with local execution off and every connected computer Manual only or missing repo, enters no_eligible_computer and disables Send", async () => {
    const user = userEvent.setup();
    vi.spyOn(computersApi, "fetchComputers").mockResolvedValue({
      local: { run_agents: false, load_balancing: true, load_balancing_weight: 50 },
      computers: [{ ...STUDIO_COMPUTER, load_balancing_weight: 0 }],
    });
    vi.spyOn(computersApi, "fetchHostResources").mockResolvedValue({
      cpu_count: 12,
      cpu_util: 0.2,
      mem_total: 32_000_000_000,
      mem_available: 20_000_000_000,
      sampled_at: new Date().toISOString(),
    });
    vi.spyOn(computersApi, "previewComputerPlacement").mockRejectedValue(
      new computersApi.ComputersApiError(
        "No computer can run this project.",
        409,
        "no_eligible_computer",
        {
          local: "local_execution_disabled",
          cmp_studio: "manual_only",
        }
      )
    );

    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const store = createSessionCreateStore();
    store.trigger.dialogOpened({ agentName: "general", workspaceId: WORKSPACE_API.id });

    render(
      <SessionCreateDialogHost
        activeWorkspace={WORKSPACE_API}
        agents={[GENERAL_AGENT]}
        dataScope={{ kind: "project", workspaceId: WORKSPACE_API.id }}
        projectWorkspaceId={WORKSPACE_API.id}
        store={store}
      />,
      { wrapper: createWrapper(queryClient) }
    );

    const picker = await screen.findByTestId("session-create-run-on");
    await user.click(picker);

    // US-014 AC-1: This computer is absent when local execution is off.
    expect(screen.queryByTestId("session-create-run-on-item-local")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("session-create-run-on-item-auto"));

    await waitFor(() => {
      expect(screen.getByTestId("auto-no-eligible-alert")).toHaveTextContent(
        "No computer can run this project."
      );
    });
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
  });
});
