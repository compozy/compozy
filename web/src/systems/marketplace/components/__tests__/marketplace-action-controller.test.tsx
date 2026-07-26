import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";

import type { MarketplaceInstalledItem } from "../../hooks/use-marketplace-kind-page";
import type { MarketplaceListing } from "../../types";
import {
  marketplaceBundlePreviewFixture,
  marketplaceDetails,
  marketplaceListings,
} from "../../mocks";
import { useMarketplaceActionController } from "../use-marketplace-action-controller";
import {
  MarketplaceKindResults,
  type MarketplaceKindResultsProps,
} from "../marketplace-kind-results";
import { marketplaceKindConfig } from "../../lib/marketplace-kind-config";

const mocks = vi.hoisted(() => ({
  activateBundle: vi.fn(),
  beginAuthorize: vi.fn(),
  installExtension: vi.fn(),
  installMCP: vi.fn(),
  installSkill: vi.fn(),
  navigate: vi.fn(),
  previewBundle: vi.fn(),
  previewData: undefined as unknown,
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  updateSkill: vi.fn(),
  updateExtension: vi.fn(),
  updateBundle: vi.fn(),
  removeSkill: vi.fn(),
  removeExtension: vi.fn(),
  deleteMCP: vi.fn(),
  toggleExtension: vi.fn(),
  deactivateBundle: vi.fn(),
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

vi.mock("@/systems/skill", async () => {
  const actual = await vi.importActual<typeof import("@/systems/skill")>("@/systems/skill");
  return {
    ...actual,
    useRemoveSkillMarketplace: () => ({ mutateAsync: mocks.removeSkill }),
  };
});

vi.mock("@/systems/extensions", async () => {
  const actual =
    await vi.importActual<typeof import("@/systems/extensions")>("@/systems/extensions");
  return {
    ...actual,
    useDeactivateBundle: () => ({ mutateAsync: mocks.deactivateBundle }),
    useRemoveExtension: () => ({ mutateAsync: mocks.removeExtension }),
    useToggleExtension: () => ({ mutateAsync: mocks.toggleExtension }),
    useUpdateBundleActivation: () => ({ mutateAsync: mocks.updateBundle }),
  };
});

vi.mock("@/systems/settings", async () => {
  const actual = await vi.importActual<typeof import("@/systems/settings")>("@/systems/settings");
  const { useState } = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    MCPAuthorizeDialog: ({ scope }: { scope: string }) => (
      <output aria-label="Authorization scope">{scope}</output>
    ),
    useDeleteSettingsMCPServer: () => ({ mutateAsync: mocks.deleteMCP }),
    useMCPAuthorize: () => {
      const [phase, setPhase] = useState("idle");
      return {
        phase,
        server: phase === "idle" ? null : "linear",
        begin: null,
        error: null,
        prior: null,
        mode: null,
        isBeginning: false,
        isExchanging: false,
        isAwaiting: phase === "waiting",
        isOpen: phase !== "idle",
        beginAuthorize: (...args: unknown[]) => {
          mocks.beginAuthorize(...args);
          setPhase("waiting");
        },
        retryBegin: vi.fn(),
        enterManual: vi.fn(),
        submitManual: vi.fn(),
        acknowledgeStatus: vi.fn(),
        cancel: vi.fn(),
      };
    },
  };
});

vi.mock("../../hooks/use-marketplace-actions", () => ({
  useActivateMarketplaceBundle: () => ({
    error: null,
    isPending: false,
    mutateAsync: mocks.activateBundle,
  }),
  useInstallMarketplaceExtension: () => ({
    isPending: false,
    mutateAsync: mocks.installExtension,
  }),
  useInstallMarketplaceMCP: () => ({ mutateAsync: mocks.installMCP }),
  useInstallMarketplaceSkill: () => ({ mutateAsync: mocks.installSkill }),
  usePreviewMarketplaceBundle: () => ({
    data: mocks.previewData,
    error: null,
    isPending: false,
    mutate: mocks.previewBundle,
  }),
  useUpdateMarketplaceSkill: () => ({ mutateAsync: mocks.updateSkill }),
  useUpdateMarketplaceExtension: () => ({
    isPending: false,
    mutateAsync: mocks.updateExtension,
  }),
}));

function ActionHarness({ entry }: { entry: MarketplaceListing }) {
  const controller = useMarketplaceActionController("ws-a");
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

function BundleUpdateHarness({ item }: { item: MarketplaceInstalledItem }) {
  const controller = useMarketplaceActionController("ws-a");
  return (
    <button onClick={() => controller.handleUpdateBundle(item)} type="button">
      Update bundle
    </button>
  );
}

function InstalledBundleResultsHarness({ item }: { item: MarketplaceInstalledItem }) {
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
    <MarketplaceKindResults
      actions={controller}
      config={marketplaceKindConfig("bundle")}
      kind="bundle"
      onEditMCP={vi.fn()}
      page={page}
    />
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
      <button onClick={() => void controller.handleDeactivate(first)} type="button">
        Deactivate first
      </button>
      <button onClick={() => void controller.handleDeactivate(second)} type="button">
        Deactivate second
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
  mocks.previewData = marketplaceBundlePreviewFixture;
  mocks.activateBundle.mockResolvedValue({});
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
  mocks.installSkill.mockResolvedValue({
    skill: {
      hash: "sha256:installed-skill",
      name: "installed-skill",
      path: "/skills/installed-skill",
      registry: "agh",
      slug: "agh/installed-skill",
      status: "installed",
      version: "1.0.0",
    },
  });
  mocks.updateExtension.mockResolvedValue(undefined);
  mocks.updateBundle.mockResolvedValue(undefined);
  mocks.updateSkill.mockResolvedValue({});
  mocks.removeSkill.mockResolvedValue({});
  mocks.removeExtension.mockResolvedValue({});
  mocks.toggleExtension.mockResolvedValue({});
  mocks.deactivateBundle.mockResolvedValue({});
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
          type: "oauth2_pkce",
          client_id: "linear-client",
          client_secret_configured: false,
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
            scope: "global",
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

    await waitFor(() => expect(mocks.beginAuthorize).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("status", { name: "Authorization scope" })).toHaveTextContent("global");
    expect(mocks.beginAuthorize).toHaveBeenCalledWith("linear", {
      status: "needs_login",
      tokenPresent: false,
    });
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
          effective_source: { kind: "global-mcp-sidecar", scope: "global" },
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
        filter: { scope: "global", target: "sidecar" },
        name: "github",
      })
    );
    expect(mocks.toastSuccess).toHaveBeenCalledWith("github removed · restart required");
  });

  it("Should track installed bundle actions by activation identity", async () => {
    const user = userEvent.setup();
    let resolveFirst: (() => void) | undefined;
    let resolveSecond: (() => void) | undefined;
    mocks.deactivateBundle
      .mockReturnValueOnce(new Promise<void>(resolve => (resolveFirst = resolve)))
      .mockReturnValueOnce(new Promise<void>(resolve => (resolveSecond = resolve)));
    const entry = { ...marketplaceListings.bundle[0]!, installed: true };
    const first = { activationId: "activation-a", entry };
    const second = { activationId: "activation-b", entry };

    render(
      <QueryClientProvider client={new QueryClient()}>
        <ConcurrentInstalledHarness first={first} second={second} />
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Deactivate first" }));
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "First installed pending" })).toHaveTextContent(
        "pending"
      )
    );
    expect(screen.getByRole("status", { name: "Second installed pending" })).toHaveTextContent(
      "idle"
    );

    await user.click(screen.getByRole("button", { name: "Deactivate second" }));
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

  it("Should update installed bundles with their current activation version", async () => {
    const user = userEvent.setup();
    const entry = {
      ...marketplaceListings.bundle[0]!,
      installed: true,
      update_available: true,
    };
    render(
      <QueryClientProvider client={new QueryClient()}>
        <BundleUpdateHarness
          item={{
            activationId: "activation-dep-kit",
            activationVersion: 7,
            entry,
          }}
        />
      </QueryClientProvider>
    );

    await user.click(screen.getByRole("button", { name: "Update bundle" }));

    await waitFor(() =>
      expect(mocks.updateBundle).toHaveBeenCalledWith({
        body: { expected_version: 7 },
        id: "activation-dep-kit",
      })
    );
  });

  it("Should disable the real installed bundle card while its update is pending", async () => {
    const user = userEvent.setup();
    let resolveUpdate: (() => void) | undefined;
    mocks.updateBundle.mockReturnValueOnce(
      new Promise<void>(resolve => {
        resolveUpdate = resolve;
      })
    );
    const item: MarketplaceInstalledItem = {
      activationId: "activation-dep-kit",
      activationVersion: 7,
      entry: {
        ...marketplaceListings.bundle[0]!,
        installed: true,
        update_available: true,
      },
    };

    render(
      <QueryClientProvider client={new QueryClient()}>
        <InstalledBundleResultsHarness item={item} />
      </QueryClientProvider>
    );

    const update = screen.getByRole("button", { name: "Update" });
    await user.click(update);

    await waitFor(() => expect(update).toBeDisabled());
    await user.click(update);
    expect(mocks.updateBundle).toHaveBeenCalledTimes(1);

    await act(async () => resolveUpdate?.());
    await waitFor(() => expect(update).toBeEnabled());
  });

  it("Should install and update skills while clearing pending state", async () => {
    const user = userEvent.setup();
    const { rerender } = setup(marketplaceListings.skill[1]!);

    await user.click(screen.getByRole("button", { name: "Run action" }));
    await waitFor(() =>
      expect(mocks.installSkill).toHaveBeenCalledWith({
        slug: "agh/docs-sync",
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
      search: { tab: "installed" },
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
    await user.click(await screen.findByTestId("extension-trust-confirm"));

    await waitFor(() =>
      expect(mocks.updateExtension).toHaveBeenLastCalledWith({
        body: { allow_unverified: true, version: "1.1.4" },
        name: "manifest-slack-notify",
      })
    );
    expect(mocks.installExtension).not.toHaveBeenCalled();
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
    await user.click(screen.getByRole("button", { name: "Run action" }));
    await waitFor(() =>
      expect(mocks.installExtension).toHaveBeenCalledWith({
        allow_unverified: false,
        slug: "agh/otel-bridge",
        version: "0.6.0",
      })
    );
    const verifiedToast = mocks.toastSuccess.mock.calls.find(
      call => call[0] === "otel-bridge installed"
    );
    verifiedToast?.[1].action.onClick();
    expect(mocks.navigate).toHaveBeenCalledWith({
      search: { tab: "installed" },
      to: "/marketplace/extensions",
    });

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ActionHarness entry={marketplaceListings.extension[1]!} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("extension-trust-dialog")).toBeInTheDocument();
    mocks.installExtension.mockRejectedValueOnce(new Error("policy changed"));
    await user.click(screen.getByTestId("extension-trust-confirm"));
    expect(await screen.findByRole("alert")).toHaveTextContent("policy changed");

    await user.click(screen.getByTestId("extension-trust-confirm"));
    await waitFor(() =>
      expect(screen.queryByTestId("extension-trust-dialog")).not.toBeInTheDocument()
    );
    expect(mocks.installExtension).toHaveBeenLastCalledWith({
      allow_unverified: true,
      slug: "community/slack-notify",
      version: "1.1.4",
    });
    const warningToast = mocks.toastSuccess.mock.calls.find(
      call => call[0] === "slack-notify installed"
    );
    warningToast?.[1].action.onClick();
    expect(mocks.navigate).toHaveBeenLastCalledWith({
      search: { tab: "installed" },
      to: "/marketplace/extensions",
    });
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

    await user.click(screen.getByRole("button", { name: "Run first" }));
    await user.click(screen.getByRole("button", { name: "Run second" }));
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "First pending" })).toHaveTextContent("pending")
    );
    expect(screen.getByRole("status", { name: "Second pending" })).toHaveTextContent("pending");

    await act(async () => {
      resolveSkill?.({
        skill: {
          hash: "sha256:skill",
          name: "installed-skill",
          path: "/skills/installed-skill",
          registry: "agh",
          slug: "agh/installed-skill",
          status: "installed",
          version: "1.0.0",
        },
      });
    });
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "First pending" })).toHaveTextContent("idle")
    );
    expect(screen.getByRole("status", { name: "Second pending" })).toHaveTextContent("pending");

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
    await waitFor(() =>
      expect(screen.getByRole("status", { name: "Second pending" })).toHaveTextContent("idle")
    );
  });

  it("Should fetch detail before opening acquisition dialogs and confirm Live bundles", async () => {
    const user = userEvent.setup();
    const { client, rerender } = setup(marketplaceListings.mcp[0]!);
    const fetchDetail = vi.spyOn(client, "fetchQuery");
    fetchDetail.mockResolvedValueOnce(marketplaceDetails["mcp:github"]!);

    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("mcp-install-dialog")).toBeInTheDocument();
    expect(fetchDetail).toHaveBeenCalledWith(
      expect.objectContaining({ queryKey: expect.arrayContaining(["mcp", "github", "ws-a"]) })
    );

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByTestId("mcp-install-dialog")).not.toBeInTheDocument());
    fetchDetail.mockResolvedValueOnce(marketplaceDetails["bundle:dep-kit"]!);
    rerender(
      <QueryClientProvider client={client}>
        <ActionHarness entry={marketplaceListings.bundle[0]!} />
      </QueryClientProvider>
    );
    await user.click(screen.getByRole("button", { name: "Run action" }));
    expect(await screen.findByTestId("bundle-activation-dialog")).toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: /Global/ }));
    await user.click(screen.getByRole("switch", { name: "Confirm Live network participation" }));
    await user.click(screen.getByTestId("bundle-activate-confirm"));

    await waitFor(() =>
      expect(mocks.activateBundle).toHaveBeenCalledWith(
        expect.objectContaining({
          confirm_network_requirement: true,
          scope: "global",
        })
      )
    );
    expect(screen.queryByTestId("bundle-activation-dialog")).not.toBeInTheDocument();
  });
});
