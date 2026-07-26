import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { toast } from "sonner";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));
vi.mock("../../adapters/marketplace-actions-api", () => ({
  activateMarketplaceBundle: vi.fn(),
  installMarketplaceExtension: vi.fn(),
  installMarketplaceMCP: vi.fn(),
  installMarketplaceSkill: vi.fn(),
  previewMarketplaceBundle: vi.fn(),
  refreshMarketplaceCatalog: vi.fn(),
  updateMarketplaceSkill: vi.fn(),
}));
vi.mock("@/systems/extensions/adapters/extensions-api", () => ({
  updateExtension: vi.fn(),
}));

import { updateExtension } from "@/systems/extensions/adapters/extensions-api";
import {
  activateMarketplaceBundle,
  installMarketplaceExtension,
  installMarketplaceMCP,
  installMarketplaceSkill,
  previewMarketplaceBundle,
} from "../../adapters/marketplace-actions-api";
import {
  useActivateMarketplaceBundle,
  useInstallMarketplaceExtension,
  useInstallMarketplaceMCP,
  useInstallMarketplaceSkill,
  usePreviewMarketplaceBundle,
  useUpdateMarketplaceExtension,
} from "../use-marketplace-actions";

const locationAssign = vi.fn();

function setup() {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const invalidateQueries = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { invalidateQueries, wrapper };
}

beforeEach(() => {
  vi.stubGlobal("location", { assign: locationAssign });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useInstallMarketplaceMCP", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should announce required authorization and invalidate discovery and settings", async () => {
    vi.mocked(installMarketplaceMCP).mockResolvedValue({
      apply: {
        active_config_hash: "sha256:active-live",
        active_generation: 42,
        applied: true,
        apply_record_id: "cfg_apply_mcp_live_add",
        lifecycle: "live-add",
        next_action: "none",
        restart_required: false,
        scope: "global",
        warnings: [],
        write_target: "global-mcp-sidecar",
      },
      mcp_server: {
        name: "github",
        scope: "global",
        source_metadata: {
          available_targets: [],
          effective_source: { kind: "global-config", scope: "global" },
          shadowed_sources: [],
        },
        transport: "stdio",
      },
      next_step: "authorize",
    });
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceMCP(), { wrapper });
    const body = { entry_id: "github-mcp", scope: "global" as const, values: null };

    act(() => result.current.mutate(body));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(installMarketplaceMCP).toHaveBeenCalledWith(body);
    expect(toast.success).toHaveBeenCalledWith("github installed · authorization pending", {
      action: { label: "Authorize →", onClick: expect.any(Function) },
    });
    const authorizeToast = vi
      .mocked(toast.success)
      .mock.calls.find(call => call[0] === "github installed · authorization pending");
    const authorizeAction = authorizeToast?.[1] as
      | { action?: { onClick?: () => void } }
      | undefined;
    authorizeAction?.action?.onClick?.();
    expect(locationAssign).toHaveBeenCalledWith(
      "/marketplace/mcp/github-mcp?scope=global&installed_name=github"
    );
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({
      queryKey: ["settings", "collection", "mcp-servers"],
    });
  });

  it("Should announce a ready server when no authorization step remains", async () => {
    vi.mocked(installMarketplaceMCP).mockResolvedValue({
      apply: {
        active_config_hash: "sha256:active-live",
        active_generation: 42,
        applied: true,
        apply_record_id: "cfg_apply_mcp_live_add",
        lifecycle: "live-add",
        next_action: "none",
        restart_required: false,
        scope: "global",
        warnings: [],
        write_target: "global-mcp-sidecar",
      },
      mcp_server: {
        name: "filesystem",
        scope: "global",
        source_metadata: {
          available_targets: [],
          effective_source: { kind: "global-config", scope: "global" },
          shadowed_sources: [],
        },
        transport: "stdio",
      },
      next_step: "none",
    });
    const { wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceMCP(), { wrapper });

    act(() => result.current.mutate({ entry_id: "filesystem", scope: "global", values: null }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(toast.success).toHaveBeenCalledWith("filesystem installed", {
      action: { label: "View installed →", onClick: expect.any(Function) },
    });
    const manageToast = vi
      .mocked(toast.success)
      .mock.calls.find(call => call[0] === "filesystem installed");
    const manageAction = manageToast?.[1] as { action?: { onClick?: () => void } } | undefined;
    manageAction?.action?.onClick?.();
    expect(locationAssign).toHaveBeenCalledWith(
      "/marketplace/mcp/filesystem?scope=global&installed_name=filesystem"
    );
  });

  it("Should retain workspace identity in the post-install management deep link", async () => {
    vi.mocked(installMarketplaceMCP).mockResolvedValue({
      apply: {
        active_config_hash: "sha256:active-live",
        active_generation: 42,
        applied: true,
        apply_record_id: "cfg_apply_mcp_live_add",
        lifecycle: "live-add",
        next_action: "none",
        restart_required: false,
        scope: "workspace",
        warnings: [],
        workspace_id: "ws-polybot",
        write_target: "workspace-mcp-sidecar",
      },
      mcp_server: {
        name: "github",
        scope: "workspace",
        workspace_id: "ws-polybot",
        source_metadata: {
          available_targets: [],
          effective_source: {
            kind: "workspace-config",
            scope: "workspace",
            workspace_id: "ws-polybot",
          },
          shadowed_sources: [],
        },
        transport: "stdio",
      },
      next_step: "authorize",
    });
    const { wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceMCP(), { wrapper });

    act(() =>
      result.current.mutate({
        entry_id: "github-mcp",
        scope: "workspace",
        workspace_id: "ws-polybot",
        values: null,
      })
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const authorizeToast = vi
      .mocked(toast.success)
      .mock.calls.find(call => String(call[0]).includes("authorization pending"));
    const authorizeAction = authorizeToast?.[1] as
      | { action?: { onClick?: () => void } }
      | undefined;
    authorizeAction?.action?.onClick?.();
    expect(locationAssign).toHaveBeenCalledWith(
      "/marketplace/mcp/github-mcp?scope=workspace&installed_name=github&workspace_id=ws-polybot"
    );
  });
});

describe("marketplace acquisition cache boundaries", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should invalidate marketplace and installed skill caches after skill install", async () => {
    vi.mocked(installMarketplaceSkill).mockResolvedValue({
      skill: {
        hash: "sha256:reviewer",
        name: "reviewer",
        path: "/skills/reviewer",
        registry: "agh",
        slug: "@agh/reviewer",
        status: "installed",
        version: "1.2.0",
      },
    });
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceSkill(), { wrapper });

    act(() => result.current.mutate({ slug: "@agh/reviewer", version: "1.2.0" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["skills"] });
  });

  it("Should invalidate marketplace and extension management after extension install", async () => {
    vi.mocked(installMarketplaceExtension).mockResolvedValue({
      extension: {
        daemon_running: false,
        enabled: true,
        name: "review-pack",
        source: "marketplace",
        state: "installed",
        type: "native",
        version: "2.0.0",
      },
    });
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useInstallMarketplaceExtension(), { wrapper });

    act(() => result.current.mutate({ slug: "review-pack", version: "2.0.0" }));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["extensions"] });
  });

  it("Should update the canonical installed extension and reconcile both cache families", async () => {
    vi.mocked(updateExtension).mockResolvedValue(undefined);
    const { invalidateQueries, wrapper } = setup();
    const { result } = renderHook(() => useUpdateMarketplaceExtension(), { wrapper });

    act(() =>
      result.current.mutate({
        body: { allow_unverified: false, version: "2.0.0" },
        name: "manifest-review-pack",
      })
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(updateExtension).toHaveBeenCalledWith("manifest-review-pack", {
      allow_unverified: false,
      version: "2.0.0",
    });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["extensions"] });
  });

  it("Should keep bundle preview read-only and invalidate discovery only after activation", async () => {
    const activation = {
      activation: {
        bundle_name: "review-team",
        created_at: "2026-07-14T12:00:00Z",
        extension_name: "review-pack",
        id: "activation-1",
        profile_name: "strict",
        scope: "global",
        spec_drift: false,
        updated_at: "2026-07-14T12:00:00Z",
        version: 1,
      },
    };
    vi.mocked(previewMarketplaceBundle).mockResolvedValue(activation);
    vi.mocked(activateMarketplaceBundle).mockResolvedValue(activation);
    const { invalidateQueries, wrapper } = setup();
    const preview = renderHook(() => usePreviewMarketplaceBundle(), { wrapper });
    const activate = renderHook(() => useActivateMarketplaceBundle(), { wrapper });
    const body = {
      bundle_name: "review-team",
      confirm_network_requirement: true,
      extension_name: "review-pack",
      profile_name: "strict",
      scope: "global",
    };

    act(() => preview.result.current.mutate(body));
    await waitFor(() => expect(preview.result.current.isSuccess).toBe(true));
    expect(invalidateQueries).not.toHaveBeenCalled();

    act(() => activate.result.current.mutate(body));
    await waitFor(() => expect(activate.result.current.isSuccess).toBe(true));
    expect(invalidateQueries).toHaveBeenCalledWith({ queryKey: ["marketplace"] });
  });
});
