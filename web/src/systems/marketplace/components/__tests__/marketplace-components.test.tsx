import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SkillPayload } from "@/systems/skill";
import { renderWithTopbar } from "@/test/render-with-topbar";
import type { MarketplaceListing, MCPInstallResponse } from "../../types";
import { marketplaceDetails, marketplaceKindFixture, marketplaceListings } from "../../mocks";
import { MarketplaceCard } from "../marketplace-card";
import { MarketplaceDetailMeta } from "../marketplace-detail-meta";
import { MarketplaceEntryAction, MarketplaceEntryStatus } from "../marketplace-entry-actions";
import { MarketplaceGrid, MarketplaceGridSkeleton } from "../marketplace-grid";
import { MarketplaceKindPage } from "../marketplace-kind-page";
import { MCPInstallDialog } from "../mcp-install-dialog";
import { buildMCPInstallRequest } from "../mcp-install-model";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  activeWorkspaceId: "ws-a" as string | null,
  marketData: null as unknown,
  marketPages: null as unknown[] | null,
  marketOptions: vi.fn(),
  marketError: null as Error | null,
  mcpError: null as Error | null,
  marketLoading: false,
  skills: [] as SkillPayload[],
  skillsWorkspace: vi.fn(),
  extensions: [] as unknown[],
  activations: [] as unknown[],
  handleAction: vi.fn(),
  handleAuthorize: vi.fn(),
  handleUpdateBundle: vi.fn(),
  fetchNextPage: vi.fn(),
  hasNextPage: false,
  isFetchNextPageError: false,
  isFetchingNextPage: false,
  putMCP: vi.fn(),
  isEntryPending: vi.fn((_entry: MarketplaceListing) => false),
  isInstalledItemPending: vi.fn(() => false),
  isEntryFlashing: vi.fn(() => false),
  setScope: vi.fn(),
  toastSuccess: vi.fn(),
  mcpServers: [] as unknown[],
  vaultSecrets: [] as unknown[],
  createSecret: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: { success: mocks.toastSuccess },
}));

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>("@tanstack/react-router");
  return {
    ...actual,
    Link: ({
      children,
      params,
      to,
      search,
      ...props
    }: {
      children?: React.ReactNode;
      to?: string;
      params?: Record<string, string>;
      search?: Record<string, unknown>;
    } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => {
      const path = params
        ? Object.entries(params).reduce(
            (current, [key, value]) => current.replace(`$${key}`, encodeURIComponent(value)),
            to ?? ""
          )
        : (to ?? "");
      return (
        <a
          href={`${path}${
            search
              ? `?${new URLSearchParams(
                  Object.entries(search).flatMap(([key, value]) =>
                    value === undefined || value === null ? [] : [[key, String(value)]]
                  )
                ).toString()}`
              : ""
          }`}
          {...props}
        >
          {children}
        </a>
      );
    },
    useNavigate: () => mocks.navigate,
  };
});

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => ({ activeWorkspaceId: mocks.activeWorkspaceId }),
}));

vi.mock("@/systems/vault", async () => {
  const actual = await vi.importActual<typeof import("@/systems/vault")>("@/systems/vault");
  return {
    ...actual,
    usePutVaultSecret: () => ({ isPending: false, mutateAsync: mocks.createSecret }),
    useVaultSecrets: () => ({
      data: mocks.vaultSecrets,
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    }),
  };
});

vi.mock("../../hooks/use-marketplace", () => ({
  useMarketplaceKind: (options: unknown) => {
    mocks.marketOptions(options);
    return {
      data:
        mocks.marketPages || mocks.marketData
          ? {
              pageParams: (mocks.marketPages ?? [mocks.marketData]).map(() => undefined),
              pages: mocks.marketPages ?? [mocks.marketData],
            }
          : undefined,
      error: mocks.marketError,
      fetchNextPage: mocks.fetchNextPage,
      hasNextPage: mocks.hasNextPage,
      isFetching: false,
      isFetchingNextPage: mocks.isFetchingNextPage,
      isFetchNextPageError: mocks.isFetchNextPageError,
      isLoading: mocks.marketLoading,
      refetch: vi.fn(),
    };
  },
}));

vi.mock("@/systems/skill", async () => {
  const actual = await vi.importActual<typeof import("@/systems/skill")>("@/systems/skill");
  return {
    ...actual,
    useSkills: (workspace: string) => {
      mocks.skillsWorkspace(workspace);
      return {
        data: mocks.skills,
        error: null,
        isLoading: false,
        refetch: vi.fn(),
      };
    },
    useRemoveSkillMarketplace: () => ({ mutateAsync: vi.fn() }),
  };
});

vi.mock("@/systems/extensions", async () => {
  const actual =
    await vi.importActual<typeof import("@/systems/extensions")>("@/systems/extensions");
  return {
    ...actual,
    useExtensionInventory: () => ({
      data: mocks.extensions,
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    }),
    useBundleActivations: () => ({
      data: mocks.activations,
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    }),
    useDeactivateBundle: () => ({ mutateAsync: vi.fn() }),
    useRemoveExtension: () => ({ mutateAsync: vi.fn() }),
    useToggleExtension: () => ({ mutateAsync: vi.fn() }),
  };
});

vi.mock("@/systems/settings/hooks/use-settings-collections", async () => {
  const actual = await vi.importActual<
    typeof import("@/systems/settings/hooks/use-settings-collections")
  >("@/systems/settings/hooks/use-settings-collections");
  return {
    ...actual,
    useSettingsMCPServers: (filter: { scope: string }) => ({
      data: {
        mcp_servers: mocks.mcpServers.filter(
          server => (server as { scope?: string }).scope === filter.scope
        ),
      },
      error: mocks.mcpError,
      isLoading: false,
      isFetching: false,
      refetch: vi.fn(),
    }),
  };
});

vi.mock("@/systems/settings/hooks/use-settings-mutations", async () => {
  const actual = await vi.importActual<
    typeof import("@/systems/settings/hooks/use-settings-mutations")
  >("@/systems/settings/hooks/use-settings-mutations");
  return {
    ...actual,
    useDeleteSettingsMCPServer: () => ({ mutateAsync: vi.fn() }),
    usePutSettingsMCPServer: () => ({
      data: undefined,
      error: null,
      isPending: false,
      mutate: mocks.putMCP,
      reset: vi.fn(),
    }),
  };
});

vi.mock("../use-marketplace-action-controller", () => ({
  useMarketplaceActionController: () => ({
    dialogs: null,
    handleAction: mocks.handleAction,
    handleAuthorize: mocks.handleAuthorize,
    handleDeactivate: vi.fn(),
    handleRemove: vi.fn(),
    handleToggleEnabled: vi.fn(),
    handleUpdateBundle: mocks.handleUpdateBundle,
    isAuthorizing: false,
    isEntryFlashing: mocks.isEntryFlashing,
    isEntryPending: mocks.isEntryPending,
    isInstalledItemPending: mocks.isInstalledItemPending,
  }),
}));

function renderKindPage(
  kind: "skill" | "mcp" | "extension" | "bundle" = "skill",
  search: {
    config_scope?: "global" | "workspace";
    tab?: "installed";
    q?: string;
  } = {}
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const page = () => (
    <QueryClientProvider client={client}>
      <MarketplaceKindPage kind={kind} search={search} />
    </QueryClientProvider>
  );
  const view = renderWithTopbar(page());
  return { ...view, rerenderKindPage: () => view.rerender(page()) };
}

describe("MarketplaceKindPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.activeWorkspaceId = "ws-a";
    mocks.marketData = marketplaceKindFixture("skill");
    mocks.marketPages = null;
    mocks.marketError = null;
    mocks.mcpError = null;
    mocks.marketLoading = false;
    mocks.hasNextPage = false;
    mocks.isFetchNextPageError = false;
    mocks.isFetchingNextPage = false;
    mocks.skills = [];
    mocks.extensions = [];
    mocks.activations = [];
    mocks.mcpServers = [];
    mocks.vaultSecrets = [];
    mocks.createSecret.mockReset();
    mocks.isEntryPending.mockReturnValue(false);
    mocks.isInstalledItemPending.mockReturnValue(false);
  });

  it("Should render kind identity in the topbar, scope PillGroup, and marketplace cards", () => {
    renderKindPage("skill");
    expect(screen.getByRole("heading", { level: 1, name: "Skills" })).toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-kind-head-skill")).toBeNull();
    const head = document.querySelector("[data-slot='topbar']");
    const toolbar = document.querySelector("[data-slot='os-window-toolbar']");
    expect(document.querySelector("[data-slot='topbar-nav']")).toContainElement(
      screen.getByTestId("marketplace-kind-navigation")
    );
    expect(head).toContainElement(screen.getByTestId("marketplace-kind-navigation"));
    expect(toolbar).toContainElement(screen.getByTestId("marketplace-kind-search-skill"));
    expect(toolbar).not.toContainElement(screen.getByTestId("marketplace-kind-navigation"));
    expect(document.querySelector("[data-slot='topbar-actions']")).toContainElement(
      screen.getByTestId("marketplace-refresh")
    );
    expect(screen.getByTestId("marketplace-scope-skill")).toBeInTheDocument();
    expect(screen.getByTestId("marketplace-grid")).toHaveAttribute("data-view", "cards");
    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
  });

  it("Should focus Marketplace search when slash is pressed outside an editable control", async () => {
    const user = userEvent.setup();
    renderKindPage("skill");

    await user.keyboard("/");

    expect(screen.getByTestId("marketplace-kind-search-skill")).toHaveFocus();
  });

  it("Should preserve server-matched Market entries that do not contain the route query", () => {
    mocks.marketData = {
      ...marketplaceKindFixture("skill"),
      items: [marketplaceListings.skill[0]!],
      total: 1,
    };

    renderKindPage("skill", { q: "registry-ranking-signal" });

    expect(mocks.marketOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: "skill", q: "registry-ranking-signal" })
    );
    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
  });

  it("Should request the next server-owned marketplace page", async () => {
    const user = userEvent.setup();
    mocks.hasNextPage = true;
    renderKindPage("skill");

    await user.click(screen.getByRole("button", { name: "Load more" }));

    expect(mocks.fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should render every loaded cursor page with the exact server total", () => {
    const firstPage = {
      ...marketplaceKindFixture("skill"),
      items: marketplaceListings.skill.slice(0, 2),
      next_cursor: "page-2",
      total: 4,
    };
    const secondPage = {
      ...marketplaceKindFixture("skill"),
      items: marketplaceListings.skill.slice(2),
      total: 4,
    };
    mocks.marketPages = [firstPage, secondPage];

    renderKindPage("skill");

    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
    expect(screen.getByTestId("marketplace-card-spec-preflight")).toBeInTheDocument();
    expect(document.querySelector('[data-slot="topbar-count"]')).toHaveTextContent("4");
  });

  it("Should preserve loaded cards and retry only the failed continuation", async () => {
    const user = userEvent.setup();
    mocks.marketPages = [
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill.slice(0, 2),
        next_cursor: "page-2",
        total: 4,
      },
    ];
    mocks.hasNextPage = true;
    mocks.isFetchNextPageError = true;
    mocks.marketError = new Error("page 2 failed");

    renderKindPage("skill");

    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
    expect(screen.getByText("More results could not be loaded.")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should hydrate the complete catalog before rendering Installed update state", () => {
    mocks.skills = [
      {
        activation: { active: true },
        description: "QA lab bootstrap",
        dir: "/tmp/skills/qa-bootstrap",
        enabled: true,
        name: "qa-bootstrap",
        provenance: { precedence_tier: "workspace", slug: "agh/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];
    mocks.marketPages = [
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill.slice(0, 2),
        next_cursor: "page-2",
        total: 4,
      },
    ];
    mocks.hasNextPage = true;

    renderKindPage("skill", { tab: "installed" });

    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-installed-card-qa-bootstrap")).not.toBeInTheDocument();
    expect(mocks.fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should query Installed inventory without forwarding the local search to the catalog", () => {
    renderKindPage("skill", { q: "local-filter", tab: "installed" });

    expect(mocks.marketOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: "skill", q: null })
    );
  });

  it("Should block partial Installed truth and retry a failed catalog continuation", async () => {
    const user = userEvent.setup();
    mocks.skills = [
      {
        activation: { active: true },
        description: "QA lab bootstrap",
        dir: "/tmp/skills/qa-bootstrap",
        enabled: true,
        name: "qa-bootstrap",
        provenance: { precedence_tier: "workspace", slug: "agh/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];
    mocks.marketPages = [
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill.slice(0, 2),
        next_cursor: "page-2",
        total: 4,
      },
    ];
    mocks.hasNextPage = true;
    mocks.isFetchNextPageError = true;
    mocks.marketError = new Error("page 2 failed");

    renderKindPage("skill", { tab: "installed" });

    expect(screen.getByText("The marketplace catalog is incomplete")).toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-installed-card-qa-bootstrap")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(mocks.fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should join MCP inventory using its effective source identity", () => {
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpServers = [
      {
        name: "linear",
        transport: "sse",
        catalog_entry: "linear",
        scope: "workspace",
        workspace_id: "ws-a",
        auth: { type: "oauth2_pkce", client_id: "x", client_secret_configured: false },
        auth_status: {
          server_name: "linear",
          scope: "global",
          status: "needs_login",
          token_present: false,
          refreshable: true,
        },
        runtime_status: {
          configured: true,
          initialized: false,
          state: "auth_required",
          probe: "skipped",
          tool_count: 0,
        },
        source_metadata: {
          available_targets: [],
          effective_source: { kind: "global-config", scope: "global" },
          shadowed_sources: [],
        },
      },
    ];
    renderKindPage("mcp", { tab: "installed" });
    const card = screen.getByTestId("marketplace-installed-card-linear");
    expect(card).toBeInTheDocument();
    expect(within(card).getByText("sse")).toBeInTheDocument();
    expect(within(card).getByText("global")).toBeInTheDocument();
    expect(within(card).getByText("authorize")).toBeInTheDocument();
    expect(within(card).getByRole("link", { name: "View linear details" })).toHaveAttribute(
      "href",
      expect.stringMatching(/^\/marketplace\/mcp\/linear\?.*scope=global/)
    );
    expect(within(card).getByRole("link", { name: "View linear details" })).not.toHaveAttribute(
      "href",
      expect.stringContaining("workspace_id")
    );
    expect(screen.getByRole("button", { name: "Authorize" })).toBeInTheDocument();
  });

  it("Should create and edit arbitrary MCP server definitions from Installed", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpServers = [
      {
        name: "custom-local",
        transport: "stdio",
        command: "custom-mcp",
        scope: "workspace",
        workspace_id: "ws-a",
        runtime_status: {
          configured: true,
          initialized: true,
          state: "ready",
          probe: "succeeded",
          tool_count: 2,
        },
        source_metadata: {
          available_targets: ["workspace-config"],
          effective_source: {
            kind: "workspace-config",
            scope: "workspace",
            workspace_id: "ws-a",
          },
          shadowed_sources: [],
        },
      },
    ];
    renderKindPage("mcp", { tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    expect(screen.getByTestId("settings-mcp-servers-editor-title")).toHaveTextContent(
      "Add MCP server"
    );
    await user.type(screen.getByTestId("settings-mcp-servers-editor-name-input"), "new-local");
    await user.type(screen.getByTestId("settings-mcp-servers-editor-command-input"), "new-mcp");
    await user.click(screen.getByTestId("settings-mcp-servers-editor-save"));
    expect(mocks.putMCP.mock.calls[0]?.[0]).toEqual(
      expect.objectContaining({
        filter: {
          scope: "workspace",
          target: "auto",
          workspace_id: "ws-a",
        },
        name: "new-local",
      })
    );
    await user.click(screen.getByTestId("settings-mcp-servers-editor-cancel"));

    const menu = screen.getByRole("button", { name: "More for custom-local" });
    fireEvent.pointerDown(menu, { button: 0, pointerType: "mouse" });
    await user.click(menu);
    await user.click(await screen.findByRole("menuitem", { name: "Edit configuration" }));
    expect(screen.getByTestId("settings-mcp-servers-editor-title")).toHaveTextContent(
      "Edit custom-local"
    );
  });

  it("Should close the MCP editor when the active workspace changes", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    const view = renderKindPage("mcp", { tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    expect(screen.getByTestId("settings-mcp-servers-editor-title")).toBeInTheDocument();

    mocks.activeWorkspaceId = "ws-b";
    view.rerenderKindPage();

    await waitFor(() =>
      expect(screen.queryByTestId("settings-mcp-servers-editor-title")).not.toBeInTheDocument()
    );
    expect(mocks.putMCP).not.toHaveBeenCalled();
  });

  it("Should preserve sidecar or config ownership when saving installed MCP edits", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    const cases = [
      {
        expectedFilter: { scope: "workspace", target: "sidecar", workspace_id: "ws-a" },
        name: "workspace-sidecar",
        scope: "workspace",
        source: { kind: "workspace-mcp-sidecar", scope: "workspace", workspace_id: "ws-a" },
        workspace_id: "ws-a",
      },
      {
        expectedFilter: { scope: "global", target: "sidecar" },
        name: "global-sidecar",
        scope: "global",
        source: { kind: "global-mcp-sidecar", scope: "global" },
        workspace_id: undefined,
      },
      {
        expectedFilter: { scope: "workspace", target: "config", workspace_id: "ws-a" },
        name: "workspace-config",
        scope: "workspace",
        source: { kind: "workspace-config", scope: "workspace", workspace_id: "ws-a" },
        workspace_id: "ws-a",
      },
      {
        expectedFilter: { scope: "global", target: "config" },
        name: "global-config-from-workspace-collection",
        scope: "workspace",
        source: { kind: "global-config", scope: "global" },
        workspace_id: "ws-a",
      },
    ] as const;

    for (const testCase of cases) {
      mocks.putMCP.mockReset();
      mocks.mcpServers = [
        {
          command: "before-edit",
          name: testCase.name,
          runtime_status: {
            configured: true,
            initialized: true,
            probe: "succeeded",
            state: "ready",
            tool_count: 1,
          },
          scope: testCase.scope,
          source_metadata: {
            available_targets: [],
            effective_source: testCase.source,
            shadowed_sources: [],
          },
          transport: "stdio",
          ...(testCase.workspace_id ? { workspace_id: testCase.workspace_id } : {}),
        },
      ];
      const view = renderKindPage("mcp", { tab: "installed" });
      const menu = screen.getByRole("button", { name: `More for ${testCase.name}` });
      fireEvent.pointerDown(menu, { button: 0, pointerType: "mouse" });
      await user.click(menu);
      await user.click(await screen.findByRole("menuitem", { name: "Edit configuration" }));
      const command = screen.getByTestId("settings-mcp-servers-editor-command-input");
      await user.clear(command);
      await user.type(command, "after-edit");
      await user.click(screen.getByTestId("settings-mcp-servers-editor-save"));

      expect(mocks.putMCP).toHaveBeenCalledWith(
        expect.objectContaining({
          filter: testCase.expectedFilter,
          name: testCase.name,
        }),
        expect.any(Object)
      );
      view.unmount();
    }
  });

  it("Should reject a hidden same-scope MCP name before a create mutation", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpServers = [
      {
        command: "hidden-server",
        name: "hidden-server",
        scope: "workspace",
        source_metadata: {
          available_targets: [],
          effective_source: {
            kind: "workspace-config",
            scope: "workspace",
            workspace_id: "ws-a",
          },
          shadowed_sources: [],
        },
        transport: "stdio",
        workspace_id: "ws-a",
      },
    ];
    renderKindPage("mcp", { q: "no-match", tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    await user.type(screen.getByTestId("settings-mcp-servers-editor-name-input"), "hidden-server");
    await user.type(screen.getByTestId("settings-mcp-servers-editor-command-input"), "replacement");

    expect(screen.getByText('An MCP server named "hidden-server" already exists.')).toBeVisible();
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeDisabled();
    expect(mocks.putMCP).not.toHaveBeenCalled();
  });

  it("Should allow a workspace MCP override of an inherited global definition", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpServers = [
      {
        command: "global-command",
        name: "inherited-server",
        scope: "workspace",
        source_metadata: {
          available_targets: [],
          effective_source: { kind: "global-config", scope: "global" },
          shadowed_sources: [],
        },
        transport: "stdio",
        workspace_id: "ws-a",
      },
    ];
    renderKindPage("mcp", { tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    await user.type(
      screen.getByTestId("settings-mcp-servers-editor-name-input"),
      "inherited-server"
    );
    await user.type(screen.getByTestId("settings-mcp-servers-editor-command-input"), "override");

    expect(screen.queryByText(/already exists/)).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-mcp-servers-editor-save")).toBeEnabled();
  });

  it("Should report the MCP write target and lifecycle after a successful save", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.putMCP.mockImplementation((_variables, options) => {
      options.onSuccess({
        restart_required: true,
        section: "mcp-servers",
        write_target: "workspace-mcp-sidecar",
      });
    });
    renderKindPage("mcp", { tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    await user.type(screen.getByTestId("settings-mcp-servers-editor-name-input"), "feedback");
    await user.type(
      screen.getByTestId("settings-mcp-servers-editor-command-input"),
      "feedback-mcp"
    );
    await user.click(screen.getByTestId("settings-mcp-servers-editor-save"));

    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      'Saved "feedback" · workspace-mcp-sidecar · restart required'
    );
    expect(screen.queryByTestId("settings-mcp-servers-editor-title")).not.toBeInTheDocument();
  });

  it("Should preserve installed MCP rows while exposing an inventory refresh failure", () => {
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpError = new Error("MCP inventory refresh failed");
    mocks.mcpServers = [
      {
        name: "custom-local",
        transport: "stdio",
        command: "custom-mcp",
        scope: "workspace",
        workspace_id: "ws-a",
        runtime_status: {
          configured: true,
          initialized: true,
          state: "ready",
          probe: "succeeded",
          tool_count: 2,
        },
        source_metadata: {
          available_targets: ["workspace-config"],
          effective_source: {
            kind: "workspace-config",
            scope: "workspace",
            workspace_id: "ws-a",
          },
          shadowed_sources: [],
        },
      },
    ];

    renderKindPage("mcp", { tab: "installed" });

    expect(screen.getByTestId("marketplace-installed-card-custom-local")).toBeInTheDocument();
    expect(screen.getByText("MCPs results may be out of date")).toBeInTheDocument();
  });

  it("Should keep a global MCP server visible and scoped with an active workspace", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("mcp");
    mocks.mcpServers = [
      {
        name: "global-filesystem",
        transport: "stdio",
        scope: "global",
        runtime_status: {
          configured: true,
          initialized: true,
          state: "ready",
          probe: "succeeded",
          tool_count: 3,
        },
        source_metadata: {
          available_targets: [],
          effective_source: { kind: "global-config", scope: "global" },
          shadowed_sources: [],
        },
      },
    ];

    renderKindPage("mcp", { tab: "installed" });

    expect(screen.getByRole("link", { name: "View global-filesystem details" })).toHaveAttribute(
      "href",
      expect.stringMatching(/^\/marketplace\/mcp\/global-filesystem\?.*scope=global/)
    );
    const menu = screen.getByRole("button", { name: "More for global-filesystem" });
    fireEvent.pointerDown(menu, { button: 0, pointerType: "mouse" });
    await user.click(menu);
    await user.click(await screen.findByRole("menuitem", { name: "Edit configuration" }));
    expect(screen.getByText("MCP server · global")).toBeInTheDocument();
  });

  it("Should show teaching empty for Installed scope with browse CTA", async () => {
    const user = userEvent.setup();
    renderKindPage("skill", { tab: "installed" });
    expect(screen.getByTestId("marketplace-installed-empty-skill")).toBeInTheDocument();
    expect(screen.getByText(/agh skill install/)).toBeInTheDocument();
    await user.click(screen.getByTestId("marketplace-browse-market-skill"));
    expect(mocks.navigate).toHaveBeenCalled();
  });

  it("Should load global installed skills when no workspace is active", () => {
    mocks.activeWorkspaceId = null;
    mocks.skills = [
      {
        activation: {
          active: false,
          reasons: [
            {
              gate: "requires_tools",
              code: "missing_tool",
              missing: ["agh__browser_screenshot"],
              message: "gate requires_tools unmet: agh__browser_screenshot",
            },
          ],
        },
        description: "Global skill",
        dir: "/tmp/skills/global-review",
        enabled: true,
        name: "global-review",
        source: "global",
      },
    ];

    renderKindPage("skill", { tab: "installed" });

    expect(mocks.skillsWorkspace).toHaveBeenLastCalledWith("");
    const card = screen.getByTestId("marketplace-installed-card-global-review");
    expect(card).toHaveTextContent("Inactive");
    expect(card).toHaveTextContent("Missing tool: agh__browser_screenshot");
  });

  it("Should derive installed update state from later catalog pages", () => {
    mocks.marketPages = [
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill.slice(0, 2),
        next_cursor: "page-2",
        total: 4,
      },
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill.slice(2),
        total: 4,
      },
    ];
    mocks.skills = [
      {
        activation: { active: true },
        description: "QA lab bootstrap",
        dir: "/tmp/skills/qa-bootstrap",
        enabled: true,
        name: "qa-bootstrap",
        provenance: { precedence_tier: "workspace", slug: "agh/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];

    renderKindPage("skill", { tab: "installed" });

    expect(screen.getByTestId("marketplace-installed-card-qa-bootstrap")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Update" })).toBeInTheDocument();
  });

  it("Should disable an installed update while its catalog entry action is pending", () => {
    mocks.marketPages = [
      {
        ...marketplaceKindFixture("skill"),
        items: marketplaceListings.skill,
        total: marketplaceListings.skill.length,
      },
    ];
    mocks.skills = [
      {
        activation: { active: true },
        description: "QA lab bootstrap",
        dir: "/tmp/skills/qa-bootstrap",
        enabled: true,
        name: "qa-bootstrap",
        provenance: { precedence_tier: "workspace", slug: "agh/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];
    mocks.isEntryPending.mockImplementation(
      entry => (entry as { entry_id: string }).entry_id === "qa-bootstrap"
    );

    renderKindPage("skill", { tab: "installed" });

    expect(screen.getByRole("button", { name: "Update" })).toBeDisabled();
  });

  it("Should match installed skills by metadata tag", () => {
    mocks.skills = [
      {
        activation: { active: true },
        description: "Review production changes",
        dir: "/tmp/skills/reviewer",
        enabled: true,
        metadata: { tags: ["security"] },
        name: "reviewer",
        source: "workspace",
      },
    ];

    renderKindPage("skill", { q: "security", tab: "installed" });

    expect(screen.getByTestId("marketplace-installed-card-reviewer")).toBeInTheDocument();
  });

  it("Should link bundle-managed skills to their activation", async () => {
    const user = userEvent.setup();
    mocks.skills = [
      {
        activation: { active: true },
        description: "Bundled skill",
        dir: "/tmp/skills/bundled-skill",
        enabled: true,
        name: "bundled-skill",
        provenance: {
          installed_from_bundle: "activation-ops-starter",
          precedence_tier: "workspace",
        },
        source: "workspace",
      },
    ];
    mocks.activations = [
      {
        bundle_name: "ops-starter",
        extension_name: "ops-extension",
        id: "activation-ops-starter",
        profile_name: "default",
        scope: "workspace",
      },
    ];
    renderKindPage("skill", { tab: "installed" });

    const trigger = screen.getByRole("button", { name: "More for bundled-skill" });
    fireEvent.pointerDown(trigger, { button: 0, pointerType: "mouse" });
    await user.click(trigger);
    expect(await screen.findByText("ops-starter/default")).toBeInTheDocument();
    expect(await screen.findByRole("menuitem", { name: "Open bundle activation" })).toHaveAttribute(
      "href",
      "/marketplace/bundles/activations/activation-ops-starter"
    );
  });

  it("Should join installed bundles by extension and bundle identity", () => {
    mocks.marketData = {
      ...marketplaceKindFixture("bundle"),
      items: [
        {
          ...marketplaceListings.bundle[0]!,
          description: "Bundle from extension A",
          entry_id: "bundle-extension-a",
          name: "shared-bundle",
          source: "extension-a",
        },
        {
          ...marketplaceListings.bundle[0]!,
          description: "Bundle from extension B",
          entry_id: "bundle-extension-b",
          name: "shared-bundle",
          source: "extension-b",
        },
      ],
      total: 2,
    };
    mocks.activations = [
      {
        bundle_name: "shared-bundle",
        extension_name: "extension-b",
        id: "activation-extension-b",
        profile_name: "default",
        scope: "global",
      },
    ];

    renderKindPage("bundle", { tab: "installed" });

    expect(screen.getByTestId("marketplace-installed-card-bundle-extension-b")).toHaveTextContent(
      "Bundle from extension B"
    );
    expect(
      screen.queryByTestId("marketplace-installed-card-bundle-extension-a")
    ).not.toBeInTheDocument();
  });

  it("Should preserve activation identity when updating an installed bundle", async () => {
    const user = userEvent.setup();
    mocks.marketData = marketplaceKindFixture("bundle");
    mocks.activations = [
      {
        bundle_name: "dep-kit",
        created_at: "2026-07-14T12:00:00Z",
        extension_name: "agh-foundations",
        id: "activation-dep-kit",
        profile_name: "default",
        scope: "global",
        spec_drift: true,
        updated_at: "2026-07-14T12:00:00Z",
        version: 7,
      },
    ];
    renderKindPage("bundle", { tab: "installed" });

    await user.click(screen.getByRole("button", { name: "Update" }));

    expect(mocks.handleUpdateBundle).toHaveBeenCalledWith(
      expect.objectContaining({
        activationId: "activation-dep-kit",
        activationVersion: 7,
      })
    );
  });

  it("Should render query-empty with clear search", () => {
    mocks.marketData = { ...marketplaceKindFixture("skill"), items: [], total: 0 };
    renderKindPage("skill", { q: "zzzz" });
    expect(screen.getByTestId("marketplace-query-empty-skill")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear search" })).toBeInTheDocument();
  });

  it("Should cancel a pending search update when the query is cleared", () => {
    vi.useFakeTimers();
    mocks.marketData = { ...marketplaceKindFixture("skill"), items: [], total: 0 };
    renderKindPage("skill", { q: "missing" });

    fireEvent.change(screen.getByTestId("marketplace-kind-search-skill"), {
      target: { value: "stale query" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Clear search" }));
    vi.runAllTimers();

    expect(mocks.navigate).toHaveBeenCalledTimes(1);
    const navigation = mocks.navigate.mock.calls[0]?.[0] as {
      search: (current: Record<string, unknown>) => Record<string, unknown>;
    };
    expect(navigation.search({ q: "missing" })).toEqual({ q: undefined });
    vi.useRealTimers();
  });

  it("Should cancel a pending search update when browser navigation changes the route query", () => {
    vi.useFakeTimers();
    const view = renderKindPage("skill", { q: "before" });

    fireEvent.change(screen.getByTestId("marketplace-kind-search-skill"), {
      target: { value: "stale local query" },
    });
    view.rerender(
      <QueryClientProvider client={new QueryClient()}>
        <MarketplaceKindPage kind="skill" search={{ q: "browser-back-query" }} />
      </QueryClientProvider>
    );
    vi.runAllTimers();

    expect(mocks.navigate).not.toHaveBeenCalled();
    expect(screen.getByTestId("marketplace-kind-search-skill")).toHaveValue("browser-back-query");
    vi.useRealTimers();
  });
});

describe("MCP guided install", () => {
  it("Should serialize a required typed value into the workspace install request", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    render(
      <MCPInstallDialog
        data={marketplaceDetails["mcp:github"]!}
        onInstall={onInstall}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    const confirm = screen.getByTestId("mcp-install-confirm");
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText(/^GITHUB_TOKEN\*?$/), "github-secret");
    await user.click(confirm);

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "workspace",
        values: { env: { GITHUB_TOKEN: { value: "github-secret" } } },
        workspace_id: "ws-story",
      })
    );
  });

  it("Should bind an existing Vault ref without reading its value", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    mocks.vaultSecrets = [
      {
        created_at: "2026-07-18T12:00:00Z",
        kind: "mcp_env",
        namespace: "mcp",
        present: true,
        ref: "vault:mcp/ws/ws-story/github/env/GITHUB_TOKEN",
        updated_at: "2026-07-18T12:00:00Z",
      },
    ];
    render(
      <MCPInstallDialog
        data={marketplaceDetails["mcp:github"]!}
        onInstall={onInstall}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    await user.click(screen.getByRole("button", { name: "Use Vault" }));
    await user.click(
      screen.getByRole("radio", { name: /vault:mcp\/ws\/ws-story\/github\/env\/GITHUB_TOKEN/ })
    );
    await user.click(screen.getByTestId("mcp-install-confirm"));

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        values: {
          env: {
            GITHUB_TOKEN: { vault_ref: "vault:mcp/ws/ws-story/github/env/GITHUB_TOKEN" },
          },
        },
      })
    );
  });

  it("Should create an inline Vault secret and install with its canonical ref", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    mocks.createSecret.mockResolvedValue(undefined);
    render(
      <MCPInstallDialog
        data={marketplaceDetails["mcp:github"]!}
        onInstall={onInstall}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    await user.click(screen.getByRole("button", { name: "Use Vault" }));
    await user.click(screen.getByRole("button", { name: "Create Vault secret" }));
    await user.type(screen.getByLabelText("New Vault value for GITHUB_TOKEN"), "created-secret");
    await user.click(screen.getByTestId("mcp-create-secret-GITHUB_TOKEN"));

    await waitFor(() => expect(mocks.createSecret).toHaveBeenCalledOnce());
    expect(mocks.createSecret).toHaveBeenCalledWith({
      kind: "mcp_env",
      ref: "vault:mcp/ws/ws-story/github/env/GITHUB_TOKEN",
      secret_value: "created-secret",
    });

    await user.click(screen.getByTestId("mcp-install-confirm"));
    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        values: {
          env: {
            GITHUB_TOKEN: { vault_ref: "vault:mcp/ws/ws-story/github/env/GITHUB_TOKEN" },
          },
        },
      })
    );
  });

  it("Should preserve bindings and expose the daemon error after install rejection", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockRejectedValue(new Error("Config write rejected"));
    const onOpenChange = vi.fn();
    render(
      <MCPInstallDialog
        data={marketplaceDetails["mcp:github"]!}
        onInstall={onInstall}
        onOpenChange={onOpenChange}
        open
        workspaceId="ws-story"
      />
    );

    const input = screen.getByLabelText(/^GITHUB_TOKEN\*?$/);
    await user.type(input, "keep-this-value");
    await user.click(screen.getByTestId("mcp-install-confirm"));

    expect(await screen.findByTestId("mcp-install-error")).toHaveTextContent(
      "Config write rejected"
    );
    expect(input).toHaveValue("keep-this-value");
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  it("Should submit remote OAuth installation without secret values", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    const remote = marketplaceDetails["mcp:linear"]!;
    render(
      <MCPInstallDialog
        data={remote}
        onInstall={onInstall}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    expect(screen.getByText("Authorization · OAuth 2 PKCE")).toBeInTheDocument();
    expect(screen.queryByText("Required configuration")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("mcp-install-confirm"));

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({ entry_id: "linear", values: null })
    );
    expect(buildMCPInstallRequest(remote, "global", null, {}, true).values).toBeNull();
  });
});

describe("Marketplace cards and actions", () => {
  it("Should dispatch install and update actions with their exact entries", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const installEntry = marketplaceListings.mcp[0]!;
    const updateEntry = marketplaceListings.skill[2]!;
    const view = render(<MarketplaceEntryAction entry={installEntry} onAction={onAction} />);

    await user.click(screen.getByRole("button", { name: `Install ${installEntry.name}` }));
    expect(onAction).toHaveBeenLastCalledWith(installEntry);

    view.rerender(<MarketplaceEntryAction entry={updateEntry} onAction={onAction} />);
    await user.click(screen.getByRole("button", { name: `Update ${updateEntry.name}` }));
    expect(onAction).toHaveBeenLastCalledWith(updateEntry);
  });

  it.each([
    [marketplaceListings.mcp[0]!, "Installing…"],
    [marketplaceListings.skill[2]!, "Updating…"],
    [marketplaceListings.bundle[0]!, "Activating…"],
  ])("Should render disabled pending action for %s", (entry, label) => {
    render(<MarketplaceEntryAction entry={entry} onAction={vi.fn()} pending />);

    expect(screen.getByRole("button", { name: new RegExp(entry.name, "i") })).toBeDisabled();
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("Should expose blocked extension action without dispatching it", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const entry = marketplaceListings.extension[2]!;
    render(<MarketplaceEntryAction entry={entry} onAction={onAction} />);

    const button = screen.getByTestId(`marketplace-action-${entry.entry_id}`);
    expect(button).toHaveAttribute("aria-disabled", "true");
    await user.click(button);
    expect(onAction).not.toHaveBeenCalled();
  });

  it.each([
    [marketplaceListings.extension[0]!, "verified"],
    [marketplaceListings.extension[1]!, "unverified · 2"],
    [marketplaceListings.extension[2]!, "blocked · 1"],
    [marketplaceListings.mcp[0]!, "curated"],
  ])("Should render marketplace trust status for %s", (entry, label) => {
    render(<MarketplaceEntryStatus entry={entry} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("Should render Manage link to Installed scope for installed entries", () => {
    render(<MarketplaceEntryAction entry={marketplaceListings.skill[0]!} onAction={vi.fn()} />);
    expect(screen.getByRole("link", { name: /Manage git-flow/i })).toHaveAttribute(
      "href",
      "/marketplace/skills?tab=installed"
    );
  });

  it("Should preserve an activation-specific manage path without repeating detail identity", () => {
    const entry = {
      ...marketplaceListings.bundle[0]!,
      installed: true,
      manage_path: "/marketplace/bundles/activations/activation-specific",
      update_available: false,
    };
    const view = render(<MarketplaceEntryAction entry={entry} onAction={vi.fn()} />);
    expect(screen.getByRole("link", { name: `Manage ${entry.name}` })).toHaveAttribute(
      "href",
      entry.manage_path
    );

    view.rerender(<MarketplaceDetailMeta entry={entry} />);
    expect(screen.getByTestId("marketplace-detail-meta")).toHaveTextContent("bundle");
    expect(screen.queryByRole("heading", { name: entry.name })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: `Manage ${entry.name}` })).not.toBeInTheDocument();
  });

  it("Should label installed bundles as active", () => {
    const entry = {
      ...marketplaceListings.bundle[0]!,
      installed: true,
      update_available: false,
    };
    render(<MarketplaceEntryStatus entry={entry} />);
    expect(screen.getByText("active")).toBeInTheDocument();
  });

  it("Should render cards-only grid skeleton", () => {
    const { container } = render(<MarketplaceGridSkeleton count={2} />);
    expect(container.querySelector('[data-view="rows"]')).toBeNull();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("Should render marketplace card linking to API-kind detail", () => {
    render(<MarketplaceCard entry={marketplaceListings.skill[1]!} onAction={vi.fn()} />);
    expect(screen.getByTestId("marketplace-card-docs-sync")).toBeInTheDocument();
  });

  it("Should render marketplace grid of cards", () => {
    render(<MarketplaceGrid entries={marketplaceListings.skill.slice(0, 2)} onAction={vi.fn()} />);
    expect(screen.getByTestId("marketplace-grid")).toHaveAttribute("data-view", "cards");
  });
});
