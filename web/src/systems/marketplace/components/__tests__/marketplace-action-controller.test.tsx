import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

import { ExtensionsApiError } from "@/systems/extensions/adapters/extensions-api";
import type { MarketplaceInstalledItem } from "../../hooks/use-marketplace-kind-page";
import type { MarketplaceListing } from "../../types";
import { marketplaceDetails, marketplaceListings } from "../../mocks";
import { useMarketplaceActionController } from "../use-marketplace-action-controller";
import {
  MarketplaceKindResults,
  type MarketplaceKindResultsProps,
} from "../marketplace-kind-results";
import { marketplaceKindConfig } from "../../lib/marketplace-kind-config";

const mocks = vi.hoisted(() => ({
  requestAuthorize: vi.fn(),
  installExtension: vi.fn(),
  installMCP: vi.fn(),
  installSkill: vi.fn(),
  navigate: vi.fn(),
  previewExtensionInstall: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  updateSkill: vi.fn(),
  updateExtension: vi.fn(),
  removeSkill: vi.fn(),
  removeExtension: vi.fn(),
  deleteMCP: vi.fn(),
  toggleExtension: vi.fn(),
}));

vi.mock("@/systems/extensions", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/extensions")>()),
  previewExtensionInstall: mocks.previewExtensionInstall,
}));

vi.mock("sonner", () => ({
  toast: { error: mocks.toastError, success: mocks.toastSuccess },
}));

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>("@tanstack/react-router");
  return {
    ...actual,
    Link: ({ children }: { children: ReactNode }) => <a href="#marketplace">{children}</a>,
    useNavigate: () => mocks.navigate,
  };
});

vi.mock("@/systems/skill/hooks/use-skill-actions", () => ({
  useRemoveSkillMarketplace: () => ({ mutateAsync: mocks.removeSkill }),
}));

vi.mock("@/systems/extensions/hooks/use-extension-actions", () => ({
  useRemoveExtension: () => ({ mutateAsync: mocks.removeExtension }),
  useToggleExtension: () => ({ mutateAsync: mocks.toggleExtension }),
}));

vi.mock("@/systems/settings/hooks/use-mcp-authorize", async importOriginal => {
  const actual =
    await importOriginal<typeof import("@/systems/settings/hooks/use-mcp-authorize")>();
  const { useState } = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    useMCPAuthorize: () => {
      const [phase, setPhase] = useState("idle");
      return {
        phase,
        server: phase === "idle" ? null : "linear",
        begin: null,
        error: null,
        prior: null,
        mode: null,
        approvedScopes: [],
        requestAuthorize: (...args: unknown[]) => {
          mocks.requestAuthorize(...args);
          setPhase("waiting");
        },
        confirmScopeEscalation: vi.fn(),
        retryBegin: vi.fn(),
        enterManual: vi.fn(),
        submitManual: vi.fn(),
        acknowledgeStatus: vi.fn(),
        cancel: vi.fn(),
      };
    },
  };
});

vi.mock("@/systems/settings/hooks/use-settings-mutations", () => ({
  useDeleteSettingsMCPServer: () => ({ mutateAsync: mocks.deleteMCP }),
}));

vi.mock("@/systems/settings/components", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/settings/components")>()),
  MCPAuthorizeDialog: ({ scope }: { scope: string }) => (
    <output aria-label="Authorization scope">{scope}</output>
  ),
}));

vi.mock("../../hooks/use-marketplace-actions", () => ({
  useInstallMarketplaceExtension: () => ({
    isPending: false,
    mutateAsync: mocks.installExtension,
  }),
  useInstallMarketplaceMCP: () => ({ mutateAsync: mocks.installMCP }),
  useInstallMarketplaceSkill: () => ({ mutateAsync: mocks.installSkill }),
  useUpdateMarketplaceSkill: () => ({ mutateAsync: mocks.updateSkill }),
  useUpdateMarketplaceExtension: () => ({
    isPending: false,
    mutateAsync: mocks.updateExtension,
  }),
}));

function ActionHarness({
  entry,
  workspaceId = "ws-a",
}: {
  entry: MarketplaceListing;
  workspaceId?: string;
}) {
  const controller = useMarketplaceActionController(workspaceId);
  return (
    <>
      <button onClick={() => controller.handleAction(entry)} type="button">
        Run action
      </button>
      <button onClick={() => controller.handleFlashEnd(entry)} type="button">
        End install flash
      </button>
      <output aria-label="Pending entry">
        {controller.isEntryPending(entry) ? "pending" : "idle"}
      </output>
      <output aria-label="Flashing entry">
        {controller.isEntryFlashing(entry) ? "flashing" : "idle"}
      </output>
      {controller.dialogs}
    </>
  );
}

function ConcurrentActionHarness({
  first,
  second,
}: {
  first: MarketplaceListing;
  second: MarketplaceListing;
}) {
  const controller = useMarketplaceActionController("ws-a");
  return (
    <>
      <button onClick={() => controller.handleAction(first)} type="button">
        Run first
      </button>
      <button onClick={() => controller.handleAction(second)} type="button">
        Run second
      </button>
      <output aria-label="First pending">
        {controller.isEntryPending(first) ? "pending" : "idle"}
      </output>
      <output aria-label="Second pending">
        {controller.isEntryPending(second) ? "pending" : "idle"}
      </output>
      {controller.dialogs}
    </>
  );
}

function InstalledExtensionResultsHarness({ item }: { item: MarketplaceInstalledItem }) {
  const controller = useMarketplaceActionController("ws-a");
  const page: MarketplaceKindResultsProps["page"] = {
    clearSearch: vi.fn(),
    error: null,
    fetchNextMarketplacePage: vi.fn(),
    hasNextMarketplacePage: false,
    installedItems: [item],
    isFetchingNextMarketplacePage: false,
    isLoading: false,
    marketEntries: [],
    marketplaceContinuationError: null,
    query: "",
    refetch: vi.fn(),
    scope: "installed",
    setScope: vi.fn(),
  };

  return (
    <>
      <MarketplaceKindResults
        actions={controller}
        config={marketplaceKindConfig("extension")}
        kind="extension"
        onEditMCP={vi.fn()}
        page={page}
      />
      {controller.dialogs}
    </>
  );
}

function AuthorizeHarness({ item }: { item: MarketplaceInstalledItem }) {
  const controller = useMarketplaceActionController("ws-a", { installedItems: [item] });
  return (
    <>
      <button onClick={() => controller.handleAuthorize(item)} type="button">
        Authorize MCP
      </button>
      {controller.dialogs}
    </>
  );
}

function RemoveHarness({ item }: { item: MarketplaceInstalledItem }) {
  const controller = useMarketplaceActionController("ws-a");
  return (
    <>
      <button onClick={() => void controller.handleRemove(item)} type="button">
        Remove installed item
      </button>
      <output aria-label="Pending installed item">
        {controller.isInstalledItemPending(item) ? "pending" : "idle"}
      </output>
    </>
  );
}

function ConcurrentInstalledHarness({
  first,
  second,
}: {
  first: MarketplaceInstalledItem;
  second: MarketplaceInstalledItem;
}) {
  const controller = useMarketplaceActionController("ws-a");
  return (
    <>
      <button onClick={() => void controller.handleRemove(first)} type="button">
        Remove first
      </button>
      <button onClick={() => void controller.handleRemove(second)} type="button">
        Remove second
      </button>
      <output aria-label="First installed pending">
        {controller.isInstalledItemPending(first) ? "pending" : "idle"}
      </output>
      <output aria-label="Second installed pending">
        {controller.isInstalledItemPending(second) ? "pending" : "idle"}
      </output>
    </>
  );
}

function setup(entry: MarketplaceListing) {
  const client = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const result = render(
    <QueryClientProvider client={client}>
      <ActionHarness entry={entry} />
    </QueryClientProvider>
  );
  return { client, ...result };
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.installExtension.mockResolvedValue({
    extension: {
      daemon_running: false,
      enabled: true,
      name: "installed-extension",
      source: "marketplace",
      state: "installed",
      type: "native",
      version: "1.0.0",
    },
  });
  mocks.installMCP.mockResolvedValue({});
  mocks.previewExtensionInstall.mockImplementation(async (request: { ref: string }) => ({
    declared_profiles: [{ create: false, credentials: [], name: "default" }],
    name: request.ref.split("/").pop() ?? request.ref,
    placements: [],
  }));
  mocks.installSkill.mockResolvedValue({
    skill: {
      hash: "sha256:installed-skill",
      name: "installed-skill",
      path: "/skills/installed-skill",
      registry: "compozy",
      slug: "compozy/installed-skill",
      status: "installed",
      version: "1.0.0",
    },
  });
  mocks.updateExtension.mockResolvedValue(undefined);
  mocks.updateSkill.mockResolvedValue({});
  mocks.removeSkill.mockResolvedValue({});
  mocks.removeExtension.mockResolvedValue({});
  mocks.toggleExtension.mockResolvedValue({});
  mocks.deleteMCP.mockResolvedValue({});
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("useMarketplaceActionController", () => {
  it("Should dispatch one authorization begin when the auth state rerenders", async () => {
    const user = userEvent.setup();
    const item: MarketplaceInstalledItem = {
      entry: marketplaceListings.mcp[1]!,
      mcpServer: {
        name: "linear",
        transport: "http",
        scope: "workspace",
        workspace_id: "ws-a",
        auth: {
          client_secret_configured: false,
          registration: "auto",
        },
        auth_status: {
          refreshable: true,
          scope: "workspace",
          server_name: "linear",
          status: "needs_login",
          token_present: false,
        },
        source_metadata: {
          available_targets: ["global-config"],
          effective_source: {
            kind: "global-config",
            scope: "user",
          },
          shadowed_sources: [],
        },
      },
    };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <AuthorizeHarness item={item} />
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Authorize MCP" }));

    await waitFor(() => expect(mocks.requestAuthorize).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("status", { name: "Authorization scope" })).toHaveTextContent("user");
    expect(mocks.requestAuthorize).toHaveBeenCalledWith({ scope: "user" }, item.mcpServer);
  });

  it("Should remove MCP servers from the exact effective source target", async () => {
    const user = userEvent.setup();
    mocks.deleteMCP.mockResolvedValueOnce({ restart_required: true });
    const item: MarketplaceInstalledItem = {
      entry: { ...marketplaceListings.mcp[0]!, installed: true, installed_name: "github" },
      mcpServer: {
        name: "github",
        transport: "stdio",
        scope: "workspace",
        workspace_id: "ws-a",
        source_metadata: {
          available_targets: ["global-mcp-sidecar"],
          effective_source: { kind: "global-mcp-sidecar", scope: "user" },
          shadowed_sources: [],
        },
      },
    };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <RemoveHarness item={item} />
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Remove installed item" }));

    await waitFor(() =>
      expect(mocks.deleteMCP).toHaveBeenCalledWith({
        filter: { scope: "user", target: "sidecar" },
        name: "github",
      })
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith("github removed · restart required");
  });

  it("Should track installed extension actions by their installed identity", async () => {
    const user = userEvent.setup();
    let resolveFirst: (() => void) | undefined;
    let resolveSecond: (() => void) | undefined;
    mocks.removeExtension
      .mockReturnValueOnce(new Promise<void>(resolve => (resolveFirst = resolve)))
      .mockReturnValueOnce(new Promise<void>(resolve => (resolveSecond = resolve)));
    const base = { ...marketplaceListings.extension[0]!, installed: true };
    const first = { entry: { ...base, installed_name: "otel-bridge" } };
    const second = { entry: { ...base, installed_name: "otel-bridge-canary" } };

    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConcurrentInstalledHarness first={first} second={second} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Remove first" }));
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "First installed pending" })).toHaveTextContent(
        "pending"
      )
    );
    expect(screen.getByRole("status", { name: "Second installed pending" })).toHaveTextContent(
      "idle"
    );

    await user.click(screen.getByRole("button", { name: "Remove second" }));
    expect(screen.getByRole("status", { name: "Second installed pending" })).toHaveTextContent(
      "pending"
    );

    await act(async () => resolveFirst?.());
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "First installed pending" })).toHaveTextContent(
        "idle"
      )
    );
    expect(screen.getByRole("status", { name: "Second installed pending" })).toHaveTextContent(
      "pending"
    );

    await act(async () => resolveSecond?.());
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "Second installed pending" })).toHaveTextContent(
        "idle"
      )
    );
  });

  it("Should disable the installed extension card while its update is pending", async () => {
    const user = userEvent.setup();
    let resolveUpdate: (() => void) | undefined;
    mocks.updateExtension.mockReturnValueOnce(
      new Promise<void>(resolve => {
        resolveUpdate = resolve;
      })
    );
    const item: MarketplaceInstalledItem = {
      entry: {
        ...marketplaceListings.extension[0]!,
        installed: true,
        installed_name: "otel-bridge",
        update_available: true,
      },
    };

    render(
      <QueryClientProvider client={new QueryClient()}>
        <InstalledExtensionResultsHarness item={item} />
      </QueryClientProvider>
    );

    const update = screen.getByRole("button", { name: "Update" });
    await user.click(update);

    await waitFor(() => expect(update).toBeDisabled());
    await user.click(update);
    expect(mocks.updateExtension).toHaveBeenCalledTimes(1);

    await act(async () => resolveUpdate?.());
    await waitFor(() => expect(update).toBeEnabled());
  });

  it("Should install and update skills while clearing pending state", async () => {
    const user = userEvent.setup();
    const { rerender } = setup(marketplaceListings.skill[1]!);

    await user.click(screen.getByRole("button", { name: "Run action" }));
    await waitFor(() =>
      expect(mocks.installSkill).toHaveBeenCalledWith({
        slug: "compozy/docs-sync",
        version: "0.9.1",
      })
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "docs-sync installed",
      expect.objectContaining({ action: expect.objectContaining({ label: "View installed →" }) })
    );
    const skillToast = mocks.toastSuccess.mock.calls.find(
      call => call[0] === "docs-sync installed"
    );
    skillToast?.[1].action.onClick();
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: {},
      to: "/marketplace/skills",
    });
    expect(screen.getByRole("status", { name: "Pending entry" })).toHaveTextContent("idle");
    expect(screen.getByRole("status", { name: "Flashing entry" })).toHaveTextContent("flashing");
    await user.click(screen.getByRole("button", { name: "End install flash" }));
    expect(screen.getByRole("status", { name: "Flashing entry" })).toHaveTextContent("idle");

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ActionHarness entry={marketplaceListings.skill[2]!} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Run action" }));
    await waitFor(() => expect(mocks.updateSkill).toHaveBeenCalledWith({ name: "qa-bootstrap" }));
    expect(mocks.toastSuccess).toHaveBeenCalledWith("qa-bootstrap updated to v2.2.0");
  });

  it("Should update extensions by installed identity through PUT semantics", async () => {
    const user = userEvent.setup();
    const verifiedUpdate: MarketplaceListing = {
      ...marketplaceListings.extension[0]!,
      installed: true,
      installed_name: "manifest-otel-bridge",
      installed_version: "0.5.0",
      name: "OpenTelemetry Bridge",
      update_available: true,
    };
    const { rerender } = setup(verifiedUpdate);

    await user.click(screen.getByRole("button", { name: "Run action" }));

    await waitFor(() =>
      expect(mocks.updateExtension).toHaveBeenCalledWith({
        body: { allow_unverified: false, version: "0.6.0" },
        name: "manifest-otel-bridge",
      })
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();

    const unverifiedUpdate: MarketplaceListing = {
      ...marketplaceListings.extension[1]!,
      installed: true,
      installed_name: "manifest-slack-notify",
      installed_version: "1.0.0",
      name: "Slack Notifications",
      update_available: true,
    };
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ActionHarness entry={unverifiedUpdate} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(
      await screen.findByRole("heading", { name: "Update Slack Notifications?" })
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Update anyway" }));

    await waitFor(() =>
      expect(mocks.updateExtension).toHaveBeenLastCalledWith({
        body: { allow_unverified: true, version: "1.1.4" },
        name: "manifest-slack-notify",
      })
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();
  });

  it("Should resume a refused quick update with the exact network digest", async () => {
    const user = userEvent.setup();
    const digest = "sha256:quick-update";
    mocks.updateExtension.mockRejectedValueOnce(
      new ExtensionsApiError("network confirmation required", 409, "daemon", {
        code: "extension_network_confirmation_required",
        currentDigest: digest,
      })
    );
    const entry: MarketplaceListing = {
      ...marketplaceListings.extension[0]!,
      installed: true,
      installed_name: "manifest-otel-bridge",
      name: "OpenTelemetry Bridge",
      update_available: true,
    };
    setup(entry);

    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("extension-network-confirm-dialog")).toBeInTheDocument();
    expect(mocks.updateExtension).toHaveBeenCalledWith({
      body: { allow_unverified: false, version: "0.6.0" },
      name: "manifest-otel-bridge",
    });

    await user.click(screen.getByTestId("extension-network-confirm-accept"));
    await waitFor(() =>
      expect(mocks.updateExtension).toHaveBeenLastCalledWith({
        body: {
          allow_unverified: false,
          confirm_network_digest: digest,
          version: "0.6.0",
        },
        name: "manifest-otel-bridge",
      })
    );
    await waitFor(() =>
      expect(screen.queryByTestId("extension-network-confirm-dialog")).not.toBeInTheDocument()
    );
  });

  it("Should toggle an installed card without a network-confirmation detour", async () => {
    const user = userEvent.setup();
    const item: MarketplaceInstalledItem = {
      entry: {
        ...marketplaceListings.extension[0]!,
        installed: true,
        installed_name: "dep-kit-ops",
        name: "Dependency Kit Ops",
        update_available: false,
      },
      extensionEnabled: false,
    };

    render(
      <QueryClientProvider client={new QueryClient()}>
        <InstalledExtensionResultsHarness item={item} />
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("switch", { name: "Enable Dependency Kit Ops" }));
    await waitFor(() =>
      expect(mocks.toggleExtension).toHaveBeenCalledWith({ enabled: true, name: "dep-kit-ops" })
    );
    expect(screen.queryByTestId("extension-network-confirm-dialog")).not.toBeInTheDocument();
  });

  it("Should surface acquisition failures and always release pending state", async () => {
    const user = userEvent.setup();
    mocks.installSkill.mockRejectedValue(new Error("registry unavailable"));
    setup(marketplaceListings.skill[1]!);

    await user.click(screen.getByRole("button", { name: "Run action" }));

    await waitFor(() => expect(mocks.toastError).toHaveBeenCalledWith("registry unavailable"));
    expect(screen.getByRole("status", { name: "Pending entry" })).toHaveTextContent("idle");
  });

  it("Should enforce blocked, verified, and warning-confirm extension decisions", async () => {
    const user = userEvent.setup();
    const { rerender } = setup(marketplaceListings.extension[2]!);

    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(mocks.installExtension).not.toHaveBeenCalled();
    expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument();

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ActionHarness entry={marketplaceListings.extension[0]!} />
      </QueryClientProvider>
    );
    mocks.previewExtensionInstall.mockResolvedValueOnce({
      declared_profiles: [{ create: true, credentials: [], name: "observability" }],
      name: "otel-bridge",
      network_requirement_digest: "sha256:otel-network",
      placements: [],
    });
    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByRole("heading", { name: "Install otel-bridge" })).toBeVisible();
    expect(screen.getByText("Creates profile observability")).toBeVisible();
    expect(mocks.installExtension).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        allow_unverified: false,
        confirm_network_digest: "sha256:otel-network",
        ref: "compozy/otel-bridge",
        source: "curated",
        version: "0.6.0",
      })
    );
    const verifiedToast = mocks.toastSuccess.mock.calls.find(
      call => call[0] === "otel-bridge installed"
    );
    verifiedToast?.[1].action.onClick();
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: {},
      to: "/marketplace/extensions",
    });

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ActionHarness entry={marketplaceListings.extension[1]!} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    mocks.previewExtensionInstall.mockRejectedValueOnce(new Error("policy changed"));
    await user.click(screen.getByTestId("extension-trust-confirm"));
    expect(await screen.findByRole("alert")).toHaveTextContent("policy changed");

    await user.click(screen.getByTestId("extension-trust-confirm"));
    await waitFor(() =>
      expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument()
    );
    expect(await screen.findByRole("heading", { name: "Install slack-notify" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Install" }));
    expect(mocks.installExtension).toHaveBeenLastCalledWith({
      allow_unverified: true,
      ref: "community/slack-notify",
      source: "curated",
      version: "1.1.4",
    });
    const warningToast = mocks.toastSuccess.mock.calls.find(
      call => call[0] === "slack-notify installed"
    );
    warningToast?.[1].action.onClick();
    expect(mocks.navigate).toHaveBeenLastCalledWith({
      search: {},
      to: "/marketplace/extensions",
    });
  });

  // IT-015 / US-016.EC-3: a portable package is synthesized into resources only and requests no host
  // permissions, so its consent gate shows the daemon's warnings and never a permission grant list.
  it("Should install a curated portable entry through consent without listing permission grants", async () => {
    const user = userEvent.setup();
    const portable = marketplaceListings.extension.find(entry => entry.format === "agent-plugin")!;
    setup(portable);

    await user.click(screen.getByRole("button", { name: "Run action" }));

    const dialog = await screen.findByTestId("extension-trust-dialog");
    expect(dialog).toHaveTextContent("Unsigned package");
    expect(dialog).not.toHaveTextContent(/permission/i);
    expect(dialog).not.toHaveTextContent(/grant/i);
    expect(mocks.installExtension).not.toHaveBeenCalled();

    await user.click(screen.getByTestId("extension-trust-confirm"));
    const installSlug = portable.install_slug;
    if (!installSlug) {
      throw new Error("portable marketplace fixture must provide install_slug");
    }
    expect(
      await screen.findByRole("heading", {
        name: `Install ${installSlug.split("/").pop()}`,
      })
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenLastCalledWith({
        allow_unverified: true,
        ref: installSlug,
        source: "curated",
        version: portable.version,
      })
    );
  });

  it("Should track overlapping entries by full kind and entry identity", async () => {
    const user = userEvent.setup();
    let resolveSkill: ((value: unknown) => void) | undefined;
    let resolveExtension: ((value: unknown) => void) | undefined;
    mocks.installSkill.mockReturnValueOnce(
      new Promise(resolve => {
        resolveSkill = resolve;
      })
    );
    mocks.installExtension.mockReturnValueOnce(
      new Promise(resolve => {
        resolveExtension = resolve;
      })
    );
    const skill = { ...marketplaceListings.skill[1]!, entry_id: "shared-entry" };
    const extension = { ...marketplaceListings.extension[0]!, entry_id: "shared-entry" };
    const client = new QueryClient({
      defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <ConcurrentActionHarness first={skill} second={extension} />
      </QueryClientProvider>
    );
    const firstPending = screen.getByRole("status", { name: "First pending" });
    const secondPending = screen.getByRole("status", { name: "Second pending" });

    await user.click(screen.getByRole("button", { name: "Run first" }));
    await user.click(screen.getByRole("button", { name: "Run second" }));
    expect(await screen.findByRole("button", { name: "Install" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Install" }));
    await waitFor(() => expect(firstPending).toHaveTextContent("pending"));
    expect(secondPending).toHaveTextContent("pending");

    await act(async () => {
      resolveSkill?.({
        skill: {
          hash: "sha256:skill",
          name: "installed-skill",
          path: "/skills/installed-skill",
          registry: "compozy",
          slug: "compozy/installed-skill",
          status: "installed",
          version: "1.0.0",
        },
      });
    });
    await waitFor(() => expect(firstPending).toHaveTextContent("idle"));
    expect(secondPending).toHaveTextContent("pending");

    await act(async () => {
      resolveExtension?.({
        extension: {
          daemon_running: false,
          enabled: true,
          name: "installed-extension",
          source: "marketplace",
          state: "installed",
          type: "native",
          version: "1.0.0",
        },
      });
    });
    await waitFor(() => expect(secondPending).toHaveTextContent("idle"));
  });

  it("Should render acquisition dialogs from canonical details for the active workspace", async () => {
    const user = userEvent.setup();
    const { client } = setup(marketplaceListings.mcp[0]!);
    const fetchDetail = vi.spyOn(client, "fetchQuery");
    fetchDetail.mockImplementationOnce(async options => {
      const detail = marketplaceDetails["mcp:github"]!;
      client.setQueryData(options.queryKey, detail);
      return detail;
    });

    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("mcp-install-dialog")).toBeInTheDocument();
    expect(fetchDetail).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: expect.arrayContaining(["mcp", "github", "ws-a"]) })
    );

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByTestId("mcp-install-dialog")).not.toBeInTheDocument());
  });
});
