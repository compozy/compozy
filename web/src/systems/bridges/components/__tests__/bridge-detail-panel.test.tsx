import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import { BridgeDetailPanel } from "@/systems/bridges/components/bridge-detail-panel";
import type { BridgeSetupProjection } from "@/systems/bridges/lib/bridge-setup";
import type {
  BridgeHealth,
  BridgeProvider,
  BridgeRoute,
  BridgeSummary,
  BridgeTarget,
  BridgeTargetsResponse,
} from "@/systems/bridges/types";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return { ...actual, useNavigate: () => vi.fn() };
});

function makeBridge(overrides: Partial<BridgeSummary> = {}): BridgeSummary {
  return {
    created_at: "2026-04-13T12:00:00Z",
    delivery_defaults: { mode: "reply", peer_id: "peer_123" },
    display_name: "Support",
    dm_policy: "allowlist",
    enabled: true,
    extension_name: "ext-telegram",
    id: "brg_support",
    notification_suppress: false,
    platform: "telegram",
    provider_config: { mode: "bot" },
    routing_policy: { include_group: true, include_peer: true, include_thread: true },
    scope: "workspace",
    status: "ready",
    updated_at: "2026-04-13T12:30:00Z",
    workspace_id: "ws_test",
    ...overrides,
  };
}

function makeHealth(overrides: Partial<BridgeHealth> = {}): BridgeHealth {
  return {
    auth_failures_total: 0,
    bridge_instance_id: "brg_support",
    delivery_backlog: 0,
    delivery_dropped_total: 0,
    delivery_failures_total: 0,
    route_count: 0,
    status: "ready",
    ...overrides,
  };
}

function makeProvider(overrides: Partial<BridgeProvider> = {}): BridgeProvider {
  return {
    config_schema: { schema: "provider-config", version: "2026-04-15" },
    display_name: "Telegram",
    enabled: true,
    extension_name: "ext-telegram",
    health: "healthy",
    platform: "telegram",
    secret_slots: [{ description: "Bot token", name: "bot_token", required: true }],
    state: "active",
    ...overrides,
  };
}

let nextRouteIndex = 1;
function makeRoute(overrides: Partial<BridgeRoute> = {}): BridgeRoute {
  const routeId = String(nextRouteIndex++).padStart(3, "0");
  return {
    agent_name: "support-agent",
    bridge_instance_id: "brg_support",
    created_at: "2026-04-13T12:00:00Z",
    last_activity_at: "2026-04-13T12:15:00Z",
    peer_id: "peer_123",
    routing_key_hash: `route_hash_${routeId}`,
    scope: "workspace",
    session_id: `sess_${routeId}`,
    updated_at: "2026-04-13T12:15:00Z",
    workspace_id: "ws_test",
    ...overrides,
  };
}

function makeTarget(overrides: Partial<BridgeTarget> = {}): BridgeTarget {
  return {
    bridge_id: "brg_support",
    canonical_route: "telegram:channel:support",
    capabilities: ["direct-send", "reply"],
    display_name: "Support room",
    last_seen_at: "2026-04-13T12:20:00Z",
    normalized: "support room",
    qualifier: "telegram",
    target_type: "channel",
    updated_at: "2026-04-13T12:20:00Z",
    ...overrides,
  };
}

function makeTargetsResponse(targets: BridgeTarget[] = [makeTarget()]): BridgeTargetsResponse {
  return {
    bridge_id: "brg_support",
    cache_stale: false,
    generated_at: "2026-04-13T12:20:00Z",
    last_successful_refresh_at: "2026-04-13T12:20:00Z",
    targets,
    total: targets.length,
  };
}

function makeProjection(overrides: Partial<BridgeSetupProjection> = {}): BridgeSetupProjection {
  return {
    checklist: [
      { id: "provider", state: "complete" },
      { id: "secrets", state: "complete" },
      { id: "webhook", state: "complete" },
      { id: "verified", state: "complete" },
      { id: "enabled", state: "complete" },
      { id: "healthy", state: "complete" },
    ],
    checksBySecretSlot: {},
    facts: {
      enabled: true,
      healthy: true,
      missingRequiredSecretSlots: [],
      providerPresent: true,
      registrationCurrent: true,
      requiredSecretSlots: ["bot_token"],
      verificationCurrent: true,
      webhookPublicUrl: "https://example.com/hooks/telegram",
    },
    globalChecks: [],
    profile: {
      checkSecretSlots: { "provider.identity": ["bot_token"] },
      dashboardUrl: "https://t.me/BotFather",
      manifest: null,
      platform: "telegram",
      webhookApplicable: true,
      webhookRegistration: "telegram",
    },
    profileIssues: [],
    ...overrides,
  };
}

function makeSetup(projection: BridgeSetupProjection | null = makeProjection()) {
  return {
    isLifecyclePending: false,
    isRegistering: false,
    isVerifying: false,
    onRegisterWebhook: vi.fn(),
    onVerify: vi.fn(),
    projection,
  };
}

function defaultProps(): ComponentProps<typeof BridgeDetailPanel> {
  return {
    bridge: makeBridge(),
    error: null,
    health: makeHealth(),
    onOpenDeliveryTest: vi.fn(),
    routes: [],
    setup: makeSetup(),
    state: { isLoading: false, isRoutesLoading: false },
  };
}

function renderPanel(overrides: Partial<ComponentProps<typeof BridgeDetailPanel>> = {}) {
  return renderWithTopbar(<BridgeDetailPanel {...defaultProps()} {...overrides} />);
}

describe("BridgeDetailPanel", () => {
  it("Should render loading, error, and empty states", () => {
    const props = defaultProps();
    const { rerender } = renderWithTopbar(
      <BridgeDetailPanel
        {...props}
        bridge={undefined}
        state={{ isLoading: true, isRoutesLoading: false }}
      />
    );
    expect(screen.getByTestId("bridge-detail-loading")).toBeInTheDocument();

    rerender(<BridgeDetailPanel {...props} bridge={undefined} error={new Error("boom")} />);
    expect(screen.getByTestId("bridge-detail-error")).toHaveTextContent("boom");

    rerender(<BridgeDetailPanel {...props} bridge={undefined} />);
    expect(screen.getByTestId("bridge-detail-empty")).toBeInTheDocument();
  });

  it("Should render only daemon-owned delivery metrics and preserve event traceability", () => {
    const route = makeRoute({ session_id: "sess_trace_123" });
    renderPanel({
      health: makeHealth({
        auth_failures_total: 2,
        delivery_backlog: 4,
        delivery_dropped_total: 3,
        delivery_failures_total: 5,
        last_success_at: "2026-04-13T12:20:00Z",
        route_count: 1,
      }),
      routes: [route],
    });

    const header = screen.getByTestId("bridge-detail-header");
    expect(header).toBeInTheDocument();
    expect(screen.getByTestId("topbar-title-text")).toHaveTextContent("Support");
    expect(header.querySelector("[data-slot='page-head']")).toBeNull();
    expect(document.querySelectorAll('[data-slot="metric"]')).toHaveLength(6);
    expect(screen.getByTestId("bridge-metric-delivery-backlog")).toHaveTextContent("4");
    expect(screen.getByTestId("bridge-metric-delivery-failures")).toHaveTextContent("5");
    expect(screen.getByTestId("bridge-metric-delivery-dropped")).toHaveTextContent("3");
    expect(screen.getByTestId("bridge-metric-auth-failures")).toHaveTextContent("2");
    expect(screen.queryByText("Events (24h)")).not.toBeInTheDocument();
    expect(screen.queryByText("Success rate")).not.toBeInTheDocument();
    expect(screen.getByTestId("bridge-route-sess_trace_123")).toHaveTextContent("sess_trace_123");
  });

  it("Should render provider-runtime fallbacks when metadata is absent", () => {
    renderPanel({
      bridge: makeBridge({ dm_policy: undefined, provider_config: undefined }),
      provider: makeProvider({ config_schema: undefined, secret_slots: undefined }),
    });
    expect(screen.getByText("Provider default")).toBeInTheDocument();
    expect(screen.getByText("No structured config schema published")).toBeInTheDocument();
    expect(
      screen.getByText("No provider runtime config stored for this bridge.")
    ).toBeInTheDocument();
  });

  it("Should withhold secret cards until provider and binding availability is known", () => {
    const props = { ...defaultProps(), provider: makeProvider(), secretBindings: [] };
    const { rerender } = renderWithTopbar(
      <BridgeDetailPanel
        {...props}
        state={{
          isLoading: false,
          isProviderLoading: true,
          isRoutesLoading: false,
          providerError: new Error("provider query failed"),
        }}
      />
    );

    expect(screen.getByTestId("bridge-provider-runtime-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("bridge-provider-runtime-unavailable")).not.toBeInTheDocument();
    expect(screen.queryByTestId("bridge-secret-binding-bot_token")).not.toBeInTheDocument();
    expect(screen.queryByText("UNBOUND")).not.toBeInTheDocument();

    rerender(
      <BridgeDetailPanel
        {...props}
        state={{
          isLoading: false,
          isRoutesLoading: false,
          providerError: new Error("Provider catalog is unavailable."),
        }}
      />
    );
    expect(screen.getByTestId("bridge-provider-runtime-unavailable")).toHaveTextContent(
      "Provider catalog is unavailable."
    );
    expect(screen.queryByTestId("bridge-secret-binding-bot_token")).not.toBeInTheDocument();

    rerender(
      <BridgeDetailPanel
        {...props}
        state={{
          isLoading: false,
          isRoutesLoading: false,
          isSecretBindingsLoading: true,
          secretBindingsError: new Error("stale binding error"),
        }}
      />
    );
    expect(screen.getByTestId("bridge-secret-bindings-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("bridge-secret-bindings-unavailable")).not.toBeInTheDocument();
    expect(screen.queryByTestId("bridge-secret-binding-bot_token")).not.toBeInTheDocument();

    rerender(
      <BridgeDetailPanel
        {...props}
        state={{
          isLoading: false,
          isRoutesLoading: false,
          secretBindingsError: new Error("Vault bindings could not be loaded."),
        }}
      />
    );
    expect(screen.getByTestId("bridge-secret-bindings-unavailable")).toHaveTextContent(
      "Vault bindings could not be loaded."
    );
    expect(screen.queryByTestId("bridge-secret-binding-bot_token")).not.toBeInTheDocument();

    rerender(<BridgeDetailPanel {...props} />);
    expect(screen.getByTestId("bridge-secret-binding-bot_token")).toHaveTextContent("UNBOUND");
  });

  it("Should render target directory rows and submit resolve requests", async () => {
    const user = userEvent.setup();
    const onQueryChange = vi.fn();
    const onResolveSubmit = vi.fn();
    renderPanel({
      targetDirectory: {
        error: null,
        isLoading: false,
        isResolving: false,
        onQueryChange,
        onResolveInputChange: vi.fn(),
        onResolveSubmit,
        query: "",
        resolveInput: "Support room",
        resolveResult: null,
        response: makeTargetsResponse(),
      },
    });

    expect(screen.getByTestId("bridge-target-directory")).toHaveTextContent("Support room");
    expect(screen.getByTestId("bridge-target-telegram:channel:support")).toHaveTextContent(
      "telegram:channel:support"
    );
    await user.type(screen.getByTestId("bridge-target-search"), "ops");
    await user.click(screen.getByTestId("bridge-target-resolve-submit"));
    expect(onQueryChange).toHaveBeenCalled();
    expect(onResolveSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should route both delivery checks through one entry point, even while disabled", async () => {
    const user = userEvent.setup();
    const onOpenDeliveryTest = vi.fn();
    renderPanel({
      bridge: makeBridge({ enabled: false, status: "disabled" }),
      health: makeHealth({ status: "disabled" }),
      onOpenDeliveryTest,
    });

    // A disabled bridge can still resolve a target, so the entry point stays
    // live; the real-send gate now lives beside the action it guards.
    const entry = screen.getByTestId("open-test-delivery-btn");
    expect(entry).toBeEnabled();
    expect(entry).toHaveTextContent("Test delivery");
    expect(screen.queryByTestId("open-send-test-btn")).toBeNull();

    await user.click(entry);
    expect(onOpenDeliveryTest).toHaveBeenCalledTimes(1);
  });

  it("Should render projected setup states and invoke verify and Telegram registration", async () => {
    const user = userEvent.setup();
    const projection = makeProjection({
      checklist: [
        { id: "provider", state: "complete" },
        { id: "secrets", state: "complete" },
        { id: "webhook", state: "action-needed" },
        { id: "verified", remediation: "Refresh the bot token.", state: "fail" },
        { id: "enabled", state: "action-needed" },
        { id: "healthy", state: "skipped" },
      ],
      facts: { ...makeProjection().facts, enabled: false },
    });
    const setup = makeSetup(projection);
    renderPanel({
      bridge: makeBridge({ enabled: false, status: "disabled" }),
      health: makeHealth({ status: "disabled" }),
      setup,
    });

    expect(screen.getByTestId("bridge-setup-item-webhook")).toHaveTextContent("Webhook registered");
    expect(screen.getByTestId("bridge-setup-item-webhook")).toHaveTextContent("NEXT");
    expect(screen.getByTestId("bridge-setup-item-verified")).toHaveTextContent("FAILED");
    expect(screen.getByTestId("bridge-setup-item-verified")).toHaveTextContent(
      "Refresh the bot token."
    );

    await user.click(screen.getByTestId("verify-bridge-btn"));
    await user.click(screen.getByTestId("register-bridge-webhook-btn"));
    expect(setup.onVerify).toHaveBeenCalledTimes(1);
    expect(setup.onRegisterWebhook).toHaveBeenCalledTimes(1);
  });

  it("Should count skipped setup items as resolved and expose one provider dashboard link", () => {
    const projection = makeProjection({
      checklist: [
        { id: "provider", state: "complete" },
        { id: "secrets", state: "complete" },
        { id: "webhook", state: "skipped" },
        { id: "verified", state: "complete" },
        { id: "enabled", state: "complete" },
        { id: "healthy", state: "skipped" },
      ],
    });
    renderPanel({ setup: makeSetup(projection) });

    expect(screen.getByTestId("bridge-setup-checklist")).toHaveTextContent("6/6");
    expect(screen.getAllByRole("link", { name: "telegram dashboard" })).toHaveLength(1);
  });

  it("Should require a disabled bridge for Telegram registration without gating lifecycle enable on evidence", () => {
    const enabledProjection = makeProjection();
    const { rerender } = renderPanel({ setup: makeSetup(enabledProjection) });

    expect(screen.getByTestId("register-bridge-webhook-btn")).toBeDisabled();
    expect(screen.getByTestId("register-bridge-webhook-btn")).toHaveTextContent("Disable first");
    expect(screen.getByTestId("bridge-setup-item-webhook")).toHaveTextContent(
      "Disable the bridge before registering its Telegram webhook."
    );

    const blockedProjection = makeProjection({
      checklist: [
        { id: "provider", state: "complete" },
        { id: "secrets", state: "action-needed" },
        { id: "webhook", state: "action-needed" },
        { id: "verified", state: "action-needed" },
        { id: "enabled", state: "action-needed" },
        { id: "healthy", state: "skipped" },
      ],
      facts: { ...makeProjection().facts, enabled: false, webhookPublicUrl: null },
    });
    rerender(
      <BridgeDetailPanel
        {...defaultProps()}
        bridge={makeBridge({ enabled: false, status: "disabled" })}
        health={makeHealth({ status: "disabled" })}
        setup={makeSetup(blockedProjection)}
      />
    );
    expect(screen.getByTestId("register-bridge-webhook-btn")).toBeDisabled();
    expect(screen.getByTestId("enable-bridge-btn")).toBeEnabled();
    expect(screen.getByTestId("setup-enable-bridge-btn")).toBeEnabled();
  });

  it("Should serialize setup controls with lifecycle and registration mutations", () => {
    const readyProjection = makeProjection({
      checklist: [
        { id: "provider", state: "complete" },
        { id: "secrets", state: "complete" },
        { id: "webhook", state: "complete" },
        { id: "verified", state: "complete" },
        { id: "enabled", state: "action-needed" },
        { id: "healthy", state: "skipped" },
      ],
      facts: { ...makeProjection().facts, enabled: false },
    });
    const { rerender } = renderPanel({
      bridge: makeBridge({ enabled: false, status: "disabled" }),
      health: makeHealth({ status: "disabled" }),
      setup: { ...makeSetup(readyProjection), isLifecyclePending: true },
    });

    expect(screen.getByTestId("verify-bridge-btn")).toBeDisabled();
    expect(screen.getByTestId("register-bridge-webhook-btn")).toBeDisabled();
    expect(screen.getByTestId("enable-bridge-btn")).toBeDisabled();
    expect(screen.getByTestId("setup-enable-bridge-btn")).toBeDisabled();

    rerender(
      <BridgeDetailPanel
        {...defaultProps()}
        bridge={makeBridge({ enabled: false, status: "disabled" })}
        health={makeHealth({ status: "disabled" })}
        setup={{ ...makeSetup(readyProjection), isRegistering: true }}
      />
    );
    expect(screen.getByTestId("enable-bridge-btn")).toBeDisabled();
    expect(screen.getByTestId("setup-enable-bridge-btn")).toBeDisabled();
    fireEvent.click(screen.getByTestId("bridge-detail-overflow"));
    expect(screen.getByTestId("edit-bridge-btn")).toHaveAttribute("aria-disabled", "true");
    expect(screen.getByTestId("restart-bridge-btn")).toHaveAttribute("aria-disabled", "true");
  });

  it("Should attach only mapped verification checks to a secret slot", () => {
    const projection = makeProjection({
      checksBySecretSlot: {
        bot_token: [
          { check: "provider.identity", remediation: "", status: "pass" },
          { check: "scope.chat:write", remediation: "Add chat:write.", status: "fail" },
        ],
      },
      globalChecks: [
        { check: "provider.latency", remediation: "Retry from a stable network.", status: "warn" },
      ],
    });
    renderPanel({ provider: makeProvider(), setup: makeSetup(projection) });

    const secretCard = screen.getByTestId("bridge-secret-binding-bot_token");
    expect(secretCard).toHaveTextContent("bot_token valid");
    expect(secretCard).toHaveTextContent("provider.identity");
    expect(secretCard).toHaveTextContent("PASS");
    expect(secretCard).toHaveTextContent("Add chat:write.");
    expect(secretCard).not.toHaveTextContent("provider.latency");
    expect(screen.getByTestId("bridge-global-check-provider.latency")).toHaveTextContent(
      "Retry from a stable network."
    );
  });

  it("Should preserve lifecycle and secret binding controls", async () => {
    const user = userEvent.setup();
    const onOpenEdit = vi.fn();
    const onRestartBridge = vi.fn();
    const onSaveSecretBinding = vi.fn();
    const onDeleteSecretBinding = vi.fn();
    const onSecretDraftChange = vi.fn();
    renderPanel({
      onDeleteSecretBinding,
      onOpenEdit,
      onRestartBridge,
      onSaveSecretBinding,
      onSecretDraftChange,
      provider: makeProvider(),
      restartRequired: true,
      secretBindings: [
        {
          binding_name: "bot_token",
          bridge_instance_id: "brg_support",
          created_at: "2026-04-13T12:00:00Z",
          kind: "bot_token",
          secret_ref: "vault:bridges/brg_support/bot_token",
          updated_at: "2026-04-13T12:10:00Z",
        },
      ],
      secretInputValues: { bot_token: "telegram-token" },
    });

    expect(screen.getByTestId("bridge-restart-required")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-secret-binding-bot_token")).toHaveTextContent("BOUND");
    await user.click(screen.getByTestId("edit-bridge-btn"));
    fireEvent.click(screen.getByTestId("bridge-detail-overflow"));
    fireEvent.click(screen.getByTestId("restart-bridge-btn"));
    await user.type(screen.getByTestId("bridge-secret-env-input-bot_token"), "X");
    await user.click(screen.getByTestId("save-bridge-secret-bot_token"));
    await user.click(screen.getByTestId("delete-bridge-secret-bot_token"));
    await user.click(screen.getByTestId("confirm-delete-bridge-secret-bot_token"));

    expect(onOpenEdit).toHaveBeenCalledTimes(1);
    expect(onRestartBridge).toHaveBeenCalledTimes(1);
    expect(onSecretDraftChange).toHaveBeenCalled();
    expect(onSaveSecretBinding).toHaveBeenCalledWith("bot_token");
    expect(onDeleteSecretBinding).toHaveBeenCalledWith("bot_token");
  });
});
