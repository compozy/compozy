// Suite: marketplace components
// Invariant: marketplace actions render and submit truthful catalog and installation state.
// Boundary IN: component interaction, form admission, and visible status projection.
// Boundary OUT: catalog pagination and HTTP transport, owned by hook and adapter suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import { MarketplaceApiError } from "../../adapters/marketplace-api-error";
import type {
  MarketplaceEntryResponse,
  MarketplaceListing,
  MCPInstallRequest,
  MCPInstallResponse,
} from "../../types";
import { marketplaceDetails, marketplaceKindFixture, marketplaceListings } from "../../mocks";
import { MarketplaceCard } from "../marketplace-card";
import { MarketplaceDetailLede } from "../marketplace-detail-lede";
import { MarketplaceEntryAction, MarketplaceEntryStatus } from "../marketplace-entry-actions";
import { MarketplaceGrid, MarketplaceGridSkeleton } from "../marketplace-grid";
import { MarketplaceInstalledCard } from "../marketplace-installed-card";
import { MarketplaceKindPage } from "../marketplace-kind-page";
import { MarketplaceActionDialogs } from "../marketplace-action-dialogs";
import { MCPInstallDialog } from "../mcp-install-dialog";
import { buildMCPInstallRequest } from "../mcp-install-model";
import type { SkillPayload } from "@/systems/skill";
import { extensionTrustFacts } from "@/systems/extensions";
import { useMCPAuthorize } from "@/systems/settings";

const mocks = vi.hoisted(() => {
  const state = {
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
    extensionInventoryEnabled: vi.fn(),
    handleAction: vi.fn(),
    handleAuthorize: vi.fn(),
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
    installExtension: vi.fn(),
  };
  return Object.assign(state, {
    marketplaceKind(options: unknown, enabled = true) {
      state.marketOptions(options, enabled);
      return {
        data:
          state.marketPages || state.marketData
            ? {
                pageParams: (state.marketPages ?? [state.marketData]).map(() => undefined),
                pages: state.marketPages ?? [state.marketData],
              }
            : undefined,
        error: state.marketError,
        fetchNextPage: state.fetchNextPage,
        hasNextPage: state.hasNextPage,
        isFetching: false,
        isFetchingNextPage: state.isFetchingNextPage,
        isFetchNextPageError: state.isFetchNextPageError,
        isLoading: state.marketLoading,
        refetch: vi.fn(),
      };
    },
    skillsQuery(workspace: string, enabled = true) {
      state.skillsWorkspace(workspace, enabled);
      return {
        data: state.skills,
        error: null,
        isLoading: false,
        refetch: vi.fn(),
      };
    },
    extensionInventory(enabled = true) {
      state.extensionInventoryEnabled(enabled);
      return {
        data: state.extensions,
        error: null,
        isLoading: false,
        refetch: vi.fn(),
      };
    },
  });
});

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
      search?:
        | Record<string, unknown>
        | ((prev: Record<string, unknown>) => Record<string, unknown>);
    } & React.AnchorHTMLAttributes<HTMLAnchorElement>) => {
      const path = params
        ? Object.entries(params).reduce(
            (current, [key, value]) => current.replace(`$${key}`, encodeURIComponent(value)),
            to ?? ""
          )
        : (to ?? "");
      const resolvedSearch = typeof search === "function" ? search({}) : search;
      return (
        <a
          href={`${path}${
            resolvedSearch
              ? `?${new URLSearchParams(
                  Object.entries(resolvedSearch).flatMap(([key, value]) =>
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

function mockActiveWorkspace() {
  return {
    activeWorkspace: mocks.activeWorkspaceId
      ? { id: mocks.activeWorkspaceId, name: "launch-hq" }
      : undefined,
    activeWorkspaceId: mocks.activeWorkspaceId,
    pending: false,
    scope: mocks.activeWorkspaceId ? ("workspace" as const) : ("global" as const),
  };
}

vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useActiveWorkspace: mockActiveWorkspace,
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: mockActiveWorkspace,
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

vi.mock("@/systems/vault/hooks/use-vault", () => ({
  useVaultSecrets: () => ({
    data: mocks.vaultSecrets,
    error: null,
    isLoading: false,
    refetch: vi.fn(),
  }),
}));

vi.mock("@/systems/vault/hooks/use-vault-actions", () => ({
  usePutVaultSecret: () => ({ isPending: false, mutateAsync: mocks.createSecret }),
}));

vi.mock("../../hooks/use-marketplace", () => ({
  useMarketplaceKind: mocks.marketplaceKind,
}));

vi.mock("@/systems/skill", async () => {
  const actual = await vi.importActual<typeof import("@/systems/skill")>("@/systems/skill");
  return {
    ...actual,
    useSkills: mocks.skillsQuery,
    useRemoveSkillMarketplace: () => ({ mutateAsync: vi.fn() }),
  };
});

vi.mock("@/systems/skill/hooks/use-skills", () => ({
  useSkills: mocks.skillsQuery,
}));

vi.mock("@/systems/extensions", async () => {
  const actual =
    await vi.importActual<typeof import("@/systems/extensions")>("@/systems/extensions");
  return {
    ...actual,
    useExtensionInventory: mocks.extensionInventory,
    useRemoveExtension: () => ({ mutateAsync: vi.fn() }),
    useToggleExtension: () => ({ mutateAsync: vi.fn() }),
  };
});

vi.mock("@/systems/extensions/hooks/use-extensions", () => ({
  useExtensionInventory: mocks.extensionInventory,
}));

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
      mutateAsync: mocks.putMCP,
      reset: vi.fn(),
    }),
  };
});

vi.mock("../../hooks/use-marketplace-actions", async () => {
  const actual = await vi.importActual<typeof import("../../hooks/use-marketplace-actions")>(
    "../../hooks/use-marketplace-actions"
  );
  return {
    ...actual,
    useInstallMarketplaceExtension: () => ({
      isPending: false,
      mutateAsync: mocks.installExtension,
    }),
  };
});

vi.mock("../use-marketplace-action-controller", () => ({
  useMarketplaceActionController: () => ({
    dialogs: null,
    handleAction: mocks.handleAction,
    handleAuthorize: mocks.handleAuthorize,
    handleRemove: vi.fn(),
    handleToggleEnabled: vi.fn(),
    isAuthorizing: false,
    isEntryFlashing: mocks.isEntryFlashing,
    isEntryPending: mocks.isEntryPending,
    isInstalledItemPending: mocks.isInstalledItemPending,
  }),
}));

function renderKindPage(
  kind: "skill" | "mcp" | "extension" = "skill",
  search: {
    tab?: "market";
    q?: string;
  } = {},
  liveDataEnabled = true
) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const page = () => (
    <QueryClientProvider client={client}>
      <MarketplaceKindPage kind={kind} liveDataEnabled={liveDataEnabled} search={search} />
    </QueryClientProvider>
  );
  const view = renderWithTopbar(page());
  return { ...view, rerenderKindPage: () => view.rerender(page()) };
}

interface MarketplaceDialogsHarnessProps {
  data: MarketplaceEntryResponse;
  onInstall: (request: MCPInstallRequest) => Promise<MCPInstallResponse>;
  scope: "global" | "workspace";
  workspaceId: string | null;
}

function MarketplaceDialogsHarness({
  data,
  onInstall,
  scope,
  workspaceId,
}: MarketplaceDialogsHarnessProps) {
  const authorize = useMCPAuthorize();
  return (
    <MarketplaceActionDialogs
      authorize={authorize}
      authScope={scope}
      authServer={null}
      mcpDetail={data}
      onConfirmTrust={() => undefined}
      onInstallMCP={onInstall}
      onMCPClose={() => undefined}
      onTrustClose={() => undefined}
      scope={scope}
      trustEntry={null}
      trustError={null}
      trustPending={false}
      workspaceId={workspaceId}
    />
  );
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
    mocks.mcpServers = [];
    mocks.vaultSecrets = [];
    mocks.createSecret.mockReset();
    mocks.installExtension.mockReset();
    mocks.installExtension.mockResolvedValue({});
    mocks.putMCP.mockImplementation(() => new Promise(() => undefined));
    mocks.isEntryPending.mockReturnValue(false);
    mocks.isInstalledItemPending.mockReturnValue(false);
  });

  it("Should open Installed when the route omits the tab search parameter", () => {
    renderKindPage("skill");

    expect(screen.getByTestId("marketplace-scope-installed-skill")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    expect(screen.getByTestId("marketplace-scope-market-skill")).toHaveAttribute(
      "aria-pressed",
      "false"
    );
  });

  it("Should suspend every Marketplace page query while its retained window is inactive", () => {
    renderKindPage("skill", {}, false);

    expect(mocks.marketOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: "skill" }),
      false
    );
    expect(mocks.skillsWorkspace).toHaveBeenLastCalledWith("ws-a", false);
    expect(mocks.extensionInventoryEnabled).toHaveBeenLastCalledWith(false);
  });

  it("Should lead the strip with views and keep the head two-element [UT-130]", () => {
    renderKindPage("skill", { tab: "market" });
    expect(screen.getByRole("heading", { level: 1, name: "Skills" })).toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-kind-head-skill")).toBeNull();
    const head = document.querySelector("[data-slot='topbar']");
    const toolbar = document.querySelector("[data-slot='os-window-toolbar']");
    // ADR-007/D3: route views are the strip's leading group, never in the head.
    expect(document.querySelector("[data-slot='topbar-nav']")).toBeNull();
    const views = screen.getByTestId("marketplace-kind-navigation");
    expect(toolbar).toContainElement(views);
    expect(head).not.toContainElement(views);
    const search = screen.getByTestId("marketplace-kind-search-skill");
    expect(toolbar).toContainElement(search);
    expect(views.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(document.querySelector("[data-slot='topbar-actions']")).toContainElement(
      screen.getByTestId("marketplace-refresh")
    );
    expect(screen.getByTestId("marketplace-scope-skill")).toBeInTheDocument();
    expect(screen.getByTestId("marketplace-grid")).toHaveAttribute("data-view", "cards");
    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
  });

  it("Should focus Marketplace search when slash is pressed outside an editable control", async () => {
    const user = userEvent.setup();
    renderKindPage("skill", { tab: "market" });

    await user.keyboard("/");

    expect(screen.getByTestId("marketplace-kind-search-skill")).toHaveFocus();
  });

  it("Should preserve server-matched Market entries that do not contain the route query", () => {
    mocks.marketData = {
      ...marketplaceKindFixture("skill"),
      items: [marketplaceListings.skill[0]!],
      total: 1,
    };

    renderKindPage("skill", { q: "registry-ranking-signal", tab: "market" });

    expect(mocks.marketOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: "skill", q: "registry-ranking-signal" }),
      true
    );
    expect(screen.getByTestId("marketplace-card-git-flow")).toBeInTheDocument();
  });

  it("Should request the next server-owned marketplace page", async () => {
    const user = userEvent.setup();
    mocks.hasNextPage = true;
    renderKindPage("skill", { tab: "market" });

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

    renderKindPage("skill", { tab: "market" });

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

    renderKindPage("skill", { tab: "market" });

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
        provenance: { precedence_tier: "workspace", slug: "compozy/qa-bootstrap" },
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

    renderKindPage("skill");

    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.queryByTestId("marketplace-installed-card-qa-bootstrap")).not.toBeInTheDocument();
    expect(mocks.fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("Should query Installed inventory without forwarding the local search to the catalog", () => {
    renderKindPage("skill", { q: "local-filter" });

    expect(mocks.marketOptions).toHaveBeenLastCalledWith(
      expect.objectContaining({ kind: "skill", q: null }),
      true
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
        provenance: { precedence_tier: "workspace", slug: "compozy/qa-bootstrap" },
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

    renderKindPage("skill");

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
        transport: "http",
        catalog_entry: "linear",
        scope: "workspace",
        workspace_id: "ws-a",
        auth: { registration: "auto" },
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
    renderKindPage("mcp");
    const card = screen.getByTestId("marketplace-installed-card-linear");
    expect(card).toBeInTheDocument();
    expect(within(card).getByText("http")).toBeInTheDocument();
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
    mocks.putMCP.mockResolvedValue({
      restart_required: false,
      section: "mcp-servers",
      write_target: "workspace-config",
    });
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
    renderKindPage("mcp");

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
    await waitFor(() => {
      expect(screen.queryByTestId("settings-mcp-servers-editor-title")).not.toBeInTheDocument();
    });

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
    const view = renderKindPage("mcp");

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
      mocks.putMCP.mockClear();
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
      const view = renderKindPage("mcp");
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
        })
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
    renderKindPage("mcp", { q: "no-match" });

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
    renderKindPage("mcp");

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
    mocks.putMCP.mockResolvedValue({
      restart_required: true,
      section: "mcp-servers",
      write_target: "workspace-mcp-sidecar",
    });
    renderKindPage("mcp");

    await user.click(screen.getByRole("button", { name: "Add MCP server" }));
    await user.type(screen.getByTestId("settings-mcp-servers-editor-name-input"), "feedback");
    await user.type(
      screen.getByTestId("settings-mcp-servers-editor-command-input"),
      "feedback-mcp"
    );
    await user.click(screen.getByTestId("settings-mcp-servers-editor-save"));

    await waitFor(() => {
      expect(mocks.toastSuccess).toHaveBeenCalledWith(
        'Saved "feedback" · workspace-mcp-sidecar · restart required'
      );
      expect(screen.queryByTestId("settings-mcp-servers-editor-title")).not.toBeInTheDocument();
    });
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

    renderKindPage("mcp");

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

    renderKindPage("mcp");

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
    renderKindPage("skill");
    expect(screen.getByTestId("marketplace-installed-empty-skill")).toBeInTheDocument();
    expect(screen.getByText(/compozy skill install/)).toBeInTheDocument();
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
              missing: ["compozy__browser_screenshot"],
              message: "gate requires_tools unmet: compozy__browser_screenshot",
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

    renderKindPage("skill");

    expect(mocks.skillsWorkspace).toHaveBeenLastCalledWith("", true);
    const card = screen.getByTestId("marketplace-installed-card-global-review");
    expect(card).toHaveTextContent("Inactive");
    expect(card).toHaveTextContent("Missing tool: compozy__browser_screenshot");
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
        provenance: { precedence_tier: "workspace", slug: "compozy/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];

    renderKindPage("skill");

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
        provenance: { precedence_tier: "workspace", slug: "compozy/qa-bootstrap" },
        source: "workspace",
        version: "2.0.0",
      },
    ];
    mocks.isEntryPending.mockImplementation(
      entry => (entry as { entry_id: string }).entry_id === "qa-bootstrap"
    );

    renderKindPage("skill");

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

    renderKindPage("skill", { q: "security" });

    expect(screen.getByTestId("marketplace-installed-card-reviewer")).toBeInTheDocument();
  });

  it("Should render query-empty with clear search", () => {
    mocks.marketData = { ...marketplaceKindFixture("skill"), items: [], total: 0 };
    renderKindPage("skill", { q: "zzzz", tab: "market" });
    expect(screen.getByTestId("marketplace-query-empty-skill")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Clear search" })).toBeInTheDocument();
  });

  it("Should cancel a pending search update when the query is cleared", () => {
    vi.useFakeTimers();
    mocks.marketData = { ...marketplaceKindFixture("skill"), items: [], total: 0 };
    renderKindPage("skill", { q: "missing", tab: "market" });

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
    const view = renderKindPage("skill", { q: "before", tab: "market" });

    fireEvent.change(screen.getByTestId("marketplace-kind-search-skill"), {
      target: { value: "stale local query" },
    });
    view.rerender(
      <QueryClientProvider client={new QueryClient()}>
        <MarketplaceKindPage kind="skill" search={{ q: "browser-back-query", tab: "market" }} />
      </QueryClientProvider>
    );
    vi.runAllTimers();

    expect(mocks.navigate).not.toHaveBeenCalled();
    expect(screen.getByTestId("marketplace-kind-search-skill")).toHaveValue("browser-back-query");
    vi.useRealTimers();
  });

  it("Should count an update once when both the catalog and the inventory report it", () => {
    const listing: MarketplaceListing = {
      ...marketplaceListings.extension[0]!,
      installed: true,
      installed_name: "otel-bridge",
      update_available: true,
    };
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [listing], total: 1 };
    mocks.extensions = [
      {
        extension: {
          consecutive_failures: 0,
          daemon_running: true,
          digest_matched: false,
          enabled: true,
          marketplace: listing,
          name: "otel-bridge",
          restart_backoff_ms: 0,
          source: "marketplace",
          state: "running",
          type: "backend",
          update_available: true,
          version: "0.5.2",
        },
        listing,
        updateAvailable: true,
      },
    ];

    renderKindPage("extension", { tab: "market" });

    expect(screen.getByTestId("marketplace-kind-updates-extension")).toHaveTextContent("1");
    expect(screen.getByTestId("marketplace-kind-meta-extension")).toHaveTextContent(
      "update available"
    );
  });

  it("Should retain the complete inventory update count while search filters visible cards", () => {
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    mocks.extensions = [
      {
        extension: {
          consecutive_failures: 0,
          daemon_running: true,
          digest_matched: false,
          enabled: true,
          name: "otel-bridge",
          restart_backoff_ms: 0,
          source: "marketplace",
          state: "running",
          type: "backend",
          update_available: true,
          version: "0.5.2",
        },
        listing: null,
        updateAvailable: true,
      },
    ];

    renderKindPage("extension", { q: "no-visible-match" });

    expect(screen.getByTestId("marketplace-kind-updates-extension")).toHaveTextContent("1");
  });

  it("Should install a local path through the source union and gate consent explicitly", async () => {
    const user = userEvent.setup();
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    renderKindPage("extension", { tab: "market" });

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "relative/dist");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "absolute path"
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.clear(screen.getByTestId("extension-install-ref"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-allow-unverified"));
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("extension-trust-confirm"));

    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        allow_unverified: true,
        ref: "/srv/hello/dist/gen-a1b2c3",
        source: "local_path",
      })
    );
  });

  it("Should reject a GitHub reference with an empty tag before the request", async () => {
    const user = userEvent.setup();
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    renderKindPage("extension", { tab: "market" });

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-source-github"));
    await user.type(screen.getByTestId("extension-install-ref"), "acme/hello@");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "owner/repo"
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();
  });

  it("Should accept only credential-free HTTPS Git repository URLs", async () => {
    const user = userEvent.setup();
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    renderKindPage("extension", { tab: "market" });

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.click(screen.getByTestId("extension-install-source-git"));
    expect(screen.getByRole("button", { name: "About repository url" })).toBeInTheDocument();

    const ref = screen.getByTestId("extension-install-ref");
    await user.type(ref, "ssh://git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "public HTTPS repository URL"
    );

    await user.clear(ref);
    await user.type(ref, "https://@git.example.com/acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Remove credentials"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com/acme/hello.git?");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "Put the Git ref in the Version field"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com./acme/hello.git");
    await user.click(screen.getByTestId("extension-install-submit"));
    expect(await screen.findByTestId("extension-install-ref-error")).toHaveTextContent(
      "valid host and repository path"
    );

    await user.clear(ref);
    await user.type(ref, "https://git.example.com/acme/hello.git@v1.2.3");
    await user.click(screen.getByTestId("extension-install-submit"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        ref: "https://git.example.com/acme/hello.git@v1.2.3",
        source: "git",
      })
    );
  });

  it("Should open consent only for the daemon checksum diagnostic", async () => {
    const user = userEvent.setup();
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    mocks.installExtension
      .mockRejectedValueOnce(
        new MarketplaceApiError(
          "Extension checksum is not registry-verified",
          422,
          "extension_checksum_unverified"
        )
      )
      .mockResolvedValueOnce({});
    renderKindPage("extension", { tab: "market" });

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    expect(mocks.installExtension).toHaveBeenCalledWith({
      ref: "/srv/hello/dist/gen-a1b2c3",
      source: "local_path",
    });

    await user.click(screen.getByTestId("extension-trust-confirm"));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenLastCalledWith({
        allow_unverified: true,
        ref: "/srv/hello/dist/gen-a1b2c3",
        source: "local_path",
      })
    );
  });

  it("Should keep a policy-blocked install on the form instead of offering consent", async () => {
    const user = userEvent.setup();
    mocks.marketData = { ...marketplaceKindFixture("extension"), items: [], total: 0 };
    mocks.installExtension.mockRejectedValueOnce(
      new MarketplaceApiError(
        "Unverified extension install is blocked by policy",
        422,
        "extension_unverified_policy_blocked"
      )
    );
    renderKindPage("extension", { tab: "market" });

    await user.click(screen.getByTestId("marketplace-extension-install"));
    await user.type(screen.getByTestId("extension-install-ref"), "/srv/hello/dist/gen-a1b2c3");
    await user.click(screen.getByTestId("extension-install-submit"));

    expect(await screen.findByTestId("extension-install-error")).toHaveTextContent(
      "blocked by policy"
    );
    expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument();
  });
});

describe("MCP guided install", () => {
  it("Should reset destination-bound inputs when an open install moves to Global", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const renderDialogs = (scope: "global" | "workspace", workspaceId: string | null) => (
      <QueryClientProvider client={client}>
        <MarketplaceDialogsHarness
          data={marketplaceDetails["mcp:github"]!}
          onInstall={onInstall}
          scope={scope}
          workspaceId={workspaceId}
        />
      </QueryClientProvider>
    );
    const view = render(renderDialogs("workspace", "ws-story"));
    const input = screen.getByLabelText(/^github_personal_access_token\*?$/);
    await user.type(input, "workspace-secret");

    view.rerender(renderDialogs("global", null));

    const resetInput = screen.getByLabelText(/^github_personal_access_token\*?$/);
    expect(resetInput).toHaveValue("");
    await user.type(resetInput, "global-secret");
    await user.click(screen.getByTestId("mcp-install-confirm"));
    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "global",
        values: { inputs: { github_personal_access_token: { value: "global-secret" } } },
        workspace_id: undefined,
      })
    );
  });

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
    await user.type(screen.getByLabelText(/^github_personal_access_token\*?$/), "github-secret");
    await user.click(confirm);

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        scope: "workspace",
        values: { inputs: { github_personal_access_token: { value: "github-secret" } } },
        workspace_id: "ws-story",
      })
    );
  });

  it("Should serialize typed identifier and boolean catalog inputs by stable input ID", async () => {
    const user = userEvent.setup();
    const onInstall = vi.fn().mockResolvedValue({} as MCPInstallResponse);
    const data: MarketplaceEntryResponse = {
      entry: {
        ...marketplaceDetails["mcp:github"]!.entry,
        entry_id: "supabase",
        name: "Supabase",
      },
      mcp: {
        default_scope: "workspace",
        inputs: [
          {
            binding: { name: "SUPABASE_PROJECT_REF", type: "env" },
            id: "project_ref",
            prompt: "Supabase project reference",
            required: true,
            type: "identifier",
          },
          {
            binding: { name: "READ_ONLY", type: "env" },
            default: true,
            id: "read_only",
            prompt: "Keep database access read-only",
            required: true,
            type: "boolean",
          },
        ],
        launch: { package: "@supabase/mcp-server-supabase", type: "npm", version: "0.6.1" },
      },
    };
    render(
      <MCPInstallDialog
        data={data}
        onInstall={onInstall}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    expect(screen.getByRole("switch", { name: "read_only" })).toBeChecked();
    await user.type(screen.getByLabelText(/^project_ref\*?$/), "project-abc");
    await user.click(screen.getByTestId("mcp-install-confirm"));

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        values: {
          inputs: {
            project_ref: { value: "project-abc" },
            read_only: { value: "true" },
          },
        },
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
        ref: "vault:mcp/ws/ws-story/github/inputs/github_personal_access_token",
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
      screen.getByRole("radio", {
        name: /vault:mcp\/ws\/ws-story\/github\/inputs\/github_personal_access_token/,
      })
    );
    await user.click(screen.getByTestId("mcp-install-confirm"));

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        values: {
          inputs: {
            github_personal_access_token: {
              vault_ref: "vault:mcp/ws/ws-story/github/inputs/github_personal_access_token",
            },
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
    await user.type(
      screen.getByLabelText("New Vault value for github_personal_access_token"),
      "created-secret"
    );
    await user.click(screen.getByTestId("mcp-create-secret-github_personal_access_token"));

    await waitFor(() => expect(mocks.createSecret).toHaveBeenCalledOnce());
    expect(mocks.createSecret).toHaveBeenCalledWith({
      kind: "mcp_env",
      ref: "vault:mcp/ws/ws-story/github/inputs/github_personal_access_token",
      secret_value: "created-secret",
    });

    await user.click(screen.getByTestId("mcp-install-confirm"));
    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({
        values: {
          inputs: {
            github_personal_access_token: {
              vault_ref: "vault:mcp/ws/ws-story/github/inputs/github_personal_access_token",
            },
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

    const input = screen.getByLabelText(/^github_personal_access_token\*?$/);
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

    expect(screen.getByText("Authorization · OAuth")).toBeInTheDocument();
    expect(screen.getByText("Automatic")).toBeInTheDocument();
    expect(screen.queryByText("Required configuration")).not.toBeInTheDocument();
    await user.click(screen.getByTestId("mcp-install-confirm"));

    await waitFor(() => expect(onInstall).toHaveBeenCalledOnce());
    expect(onInstall).toHaveBeenCalledWith(
      expect.objectContaining({ entry_id: "linear", values: null })
    );
    expect(buildMCPInstallRequest(remote, "global", null, {}).values).toBeNull();
  });

  it("Should render the catalog's pre-registered OAuth mode", () => {
    const remote = marketplaceDetails["mcp:linear"]!;
    const data: MarketplaceEntryResponse = {
      ...remote,
      mcp: {
        ...remote.mcp!,
        auth: { ...remote.mcp!.auth!, registration: "pre_registered" },
      },
    };

    render(
      <MCPInstallDialog
        data={data}
        onInstall={vi.fn().mockResolvedValue({} as MCPInstallResponse)}
        onOpenChange={vi.fn()}
        open
        workspaceId="ws-story"
      />
    );

    expect(screen.getByText("Pre-registered")).toBeInTheDocument();
    expect(screen.queryByText("Automatic")).not.toBeInTheDocument();
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
    [marketplaceListings.extension[0]!, "Installing…"],
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
    [marketplaceListings.extension[0]!, "official catalog"],
    [marketplaceListings.extension[1]!, "unverified · 2"],
    [marketplaceListings.extension[2]!, "blocked · 1"],
    [marketplaceListings.mcp[0]!, "curated"],
  ])("Should render marketplace trust status for %s", (entry, label) => {
    render(<MarketplaceEntryStatus entry={entry} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("Should reserve checksum verification status for explicit verification evidence", () => {
    const entry = {
      ...marketplaceListings.extension[0]!,
      trust: {
        ...marketplaceListings.extension[0]!.trust!,
        checksum_verified: true,
      },
    };

    render(<MarketplaceEntryStatus entry={entry} />);

    expect(screen.getByText("checksum verified")).toBeInTheDocument();
  });

  it("Should render Manage link to Installed scope for installed entries", () => {
    render(<MarketplaceEntryAction entry={marketplaceListings.skill[0]!} onAction={vi.fn()} />);
    expect(screen.getByRole("link", { name: /Manage git-flow/i })).toHaveAttribute(
      "href",
      "/marketplace/skills"
    );
  });

  it("Should preserve a daemon-supplied manage path without repeating detail identity", () => {
    const entry = {
      ...marketplaceListings.extension[0]!,
      installed: true,
      manage_path: "/marketplace/extensions",
      update_available: false,
    };
    const view = render(<MarketplaceEntryAction entry={entry} onAction={vi.fn()} />);
    expect(screen.getByRole("link", { name: `Manage ${entry.name}` })).toHaveAttribute(
      "href",
      entry.manage_path
    );

    view.rerender(<MarketplaceDetailLede data={{ entry }} />);
    expect(screen.getByTestId("marketplace-detail-lede")).toHaveTextContent("extension");
    expect(screen.getByRole("heading", { level: 1, name: entry.name })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: `Manage ${entry.name}` })).not.toBeInTheDocument();
  });

  it("Should label an installed entry as installed", () => {
    const entry = {
      ...marketplaceListings.extension[0]!,
      installed: true,
      update_available: false,
    };
    render(<MarketplaceEntryStatus entry={entry} />);
    expect(screen.getByText("installed")).toBeInTheDocument();
  });

  it("Should render cards-only grid skeleton", () => {
    const { container } = render(<MarketplaceGridSkeleton count={2} />);
    expect(container.querySelector('[data-view="rows"]')).toBeNull();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("Should render marketplace card linking to API-kind detail", () => {
    render(<MarketplaceCard entry={marketplaceListings.skill[1]!} onAction={vi.fn()} />);
    expect(
      screen.getByRole("link", { name: `View ${marketplaceListings.skill[1]!.name} details` })
    ).toHaveAttribute("href", "/marketplace/skill/docs-sync?tab=market");
  });

  it("Should render curated, installed, and update versions with exactly one prefix", () => {
    const entry = {
      ...marketplaceListings.skill[2]!,
      installed: true,
      installed_version: "v1.7.0",
      update_available: true,
      version: "V1.8.0",
    };
    const handlers = {
      onAction: vi.fn(),
      onAuthorize: vi.fn(),
      onEditMCP: vi.fn(),
      onRemove: vi.fn(),
      onToggleEnabled: vi.fn(),
    };
    const view = render(<MarketplaceCard entry={entry} onAction={handlers.onAction} />);

    expect(screen.getByTestId(`marketplace-card-${entry.entry_id}`)).toHaveTextContent("v1.8.0");

    view.rerender(<MarketplaceDetailLede data={{ entry }} />);
    expect(screen.getByTestId("marketplace-detail-lede")).toHaveTextContent("v1.8.0");

    view.rerender(<MarketplaceEntryStatus entry={entry} />);
    expect(screen.getByText("v1.8.0 available")).toBeInTheDocument();

    view.rerender(<MarketplaceInstalledCard item={{ entry }} {...handlers} />);
    expect(screen.getByTestId(`marketplace-installed-card-${entry.entry_id}`)).toHaveTextContent(
      "v1.8.0 available"
    );
  });

  it("Should render marketplace grid of cards", () => {
    render(<MarketplaceGrid entries={marketplaceListings.skill.slice(0, 2)} onAction={vi.fn()} />);
    expect(screen.getByTestId("marketplace-grid")).toHaveAttribute("data-view", "cards");
  });

  // UT-050: browsing is the only place the curated marker can speak, so the card must render it —
  // and stay silent for the native entries that carry none.
  it("Should render the format badge from a curated entry's marker and nothing without one", () => {
    const portable = marketplaceListings.extension.find(entry => entry.format === "agent-plugin")!;
    const native = marketplaceListings.extension[0]!;
    const view = render(<MarketplaceCard entry={portable} onAction={vi.fn()} />);

    expect(
      within(screen.getByTestId(`marketplace-card-${portable.entry_id}`)).getByTestId(
        "extension-format-badge"
      )
    ).toHaveTextContent("agent plugin");

    view.rerender(<MarketplaceCard entry={native} onAction={vi.fn()} />);
    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();

    view.rerender(<MarketplaceDetailLede data={{ entry: portable }} />);
    expect(
      within(screen.getByTestId("marketplace-detail-lede")).getByTestId("extension-format-badge")
    ).toBeInTheDocument();
  });

  // Once installed, the daemon's recorded format is the truth the card renders; the badge sits
  // beside the trust badges without merging into them.
  it("Should render the installed card badge from the daemon's recorded format", () => {
    const handlers = {
      onAction: vi.fn(),
      onAuthorize: vi.fn(),
      onEditMCP: vi.fn(),
      onRemove: vi.fn(),
      onToggleEnabled: vi.fn(),
    };
    const entry = {
      ...marketplaceListings.extension[0]!,
      format: "agent-plugin",
      installed: true,
      update_available: false,
    };
    const item = {
      entry,
      extensionEnabled: true,
      extensionFacts: extensionTrustFacts({
        digest_matched: true,
        installed_from: "marketplace_registry",
        registry_tier: "official",
      }),
    };
    const view = render(<MarketplaceInstalledCard item={item} {...handlers} />);

    const card = screen.getByTestId(`marketplace-installed-card-${entry.entry_id}`);
    expect(within(card).getByTestId("extension-format-badge")).toHaveTextContent("agent plugin");
    expect(within(card).getByTestId("extension-digest-matched-badge")).toBeInTheDocument();

    view.rerender(<MarketplaceDetailLede data={{ entry }} />);
    expect(
      within(screen.getByTestId("marketplace-detail-lede")).getByTestId("extension-format-badge")
    ).toHaveTextContent("agent plugin");

    view.rerender(
      <MarketplaceInstalledCard
        item={{ ...item, entry: { ...entry, format: "compozy" } }}
        {...handlers}
      />
    );
    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();
  });
});
