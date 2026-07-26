import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { EntityMode } from "@agh/ui";

import { BridgeEditDialog } from "@/systems/bridges/components/bridge-edit-dialog";
import type {
  BridgeProvider,
  BridgeTestDeliveryDraft,
  BridgeUpdateDraft,
} from "@/systems/bridges/types";

const baseDraft: BridgeUpdateDraft = {
  deliveryDefaults: {},
  dmPolicy: "allowlist",
  displayName: "Support",
  providerConfigText: '{\n  "mode": "bot"\n}',
  routingPolicy: { include_group: true, include_peer: true, include_thread: true },
};

function makeProvider(overrides: Partial<BridgeProvider> = {}): BridgeProvider {
  return {
    config_schema: {
      schema: "provider-config",
      version: "2026-04-15",
    },
    display_name: "Telegram",
    enabled: true,
    extension_name: "ext-telegram",
    health: "healthy",
    platform: "telegram",
    secret_slots: [
      {
        description: "Bot token",
        name: "bot_token",
        required: true,
      },
    ],
    state: "active",
    ...overrides,
  };
}

const identity = {
  extensionName: "ext-telegram",
  platform: "telegram",
  scope: "workspace",
  workspaceId: "ws_support",
};

const deliveryTest = {
  bridgeEnabled: true,
  draft: { message: "", target: {} },
  dryRunResult: null,
  isDryRunPending: false,
  isSendTestPending: false,
  onDraftChange: vi.fn(),
  onDryRun: vi.fn(),
  onSendTest: vi.fn(),
  sendTestResult: null,
};

/** Stateful host so mode and draft transitions behave like the real page hook. */
function DialogHarness({
  initialDraft = baseDraft,
  initialMode = "simple",
  pristineDraft = baseDraft,
  rotation,
  ...overrides
}: Partial<Parameters<typeof BridgeEditDialog>[0]> & {
  initialDraft?: BridgeUpdateDraft;
  initialMode?: EntityMode;
}) {
  const [draft, setDraft] = useState<BridgeUpdateDraft>(initialDraft);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  return (
    <BridgeEditDialog
      allowProviderDefaultDmPolicy={false}
      bridgeName="Support"
      draft={draft}
      identity={identity}
      isPending={false}
      mode={mode}
      onDraftChange={setDraft}
      onModeChange={setMode}
      onOpenChange={vi.fn()}
      onSubmit={vi.fn()}
      open
      pristineDraft={pristineDraft}
      provider={makeProvider()}
      rotation={rotation ?? { kind: "none" }}
      {...overrides}
    />
  );
}

describe("BridgeEditDialog", () => {
  it("Should present one named editor dialog with a single primary action", () => {
    render(<DialogHarness />);

    expect(screen.getByRole("dialog", { name: "Edit Support" })).toBeInTheDocument();
    expect(screen.getByTestId("submit-bridge-edit")).toHaveTextContent("Save changes");
  });

  it("Should render platform, extension and scope as immutable identity, never as inputs", () => {
    render(<DialogHarness />);

    expect(screen.getByText("telegram")).toBeInTheDocument();
    expect(screen.getByText("ext-telegram")).toBeInTheDocument();
    expect(screen.getByText("workspace")).toBeInTheDocument();
    expect(screen.queryByDisplayValue("telegram")).toBeNull();
    expect(screen.queryByDisplayValue("ext-telegram")).toBeNull();
    expect(screen.queryByDisplayValue("workspace")).toBeNull();
  });

  it("Should keep the primary inert until a mutable field actually changes", async () => {
    const user = userEvent.setup();
    render(<DialogHarness />);

    expect(screen.getByTestId("submit-bridge-edit")).toBeDisabled();

    await user.type(screen.getByTestId("bridge-edit-display-name-input"), "!");

    expect(screen.getByTestId("submit-bridge-edit")).toBeEnabled();
  });

  it("Should ignore provider-config reformatting that carries no contract change", () => {
    render(
      <DialogHarness
        initialDraft={{ ...baseDraft, providerConfigText: '{"mode":"bot"}' }}
        pristineDraft={baseDraft}
      />
    );

    expect(screen.getByTestId("submit-bridge-edit")).toBeDisabled();
  });

  it("Should treat a typed secret rotation as a change even with no PATCH field touched", () => {
    render(
      <DialogHarness
        hasPendingSecretRotation
        initialMode="advanced"
        rotation={{
          kind: "managed",
          onRotationEditingChange: vi.fn(),
          onRotationValueChange: vi.fn(),
          rotations: [{ editing: true, name: "bot_token", present: true, value: "new-token" }],
        }}
      />
    );

    expect(screen.getByTestId("submit-bridge-edit")).toBeEnabled();
  });

  it("Should hydrate a credential as presence and its reference, never as a value", () => {
    render(
      <DialogHarness
        initialMode="advanced"
        rotation={{
          kind: "managed",
          onRotationEditingChange: vi.fn(),
          onRotationValueChange: vi.fn(),
          rotations: [
            {
              editing: false,
              name: "bot_token",
              present: true,
              secretRef: "vault:bridges/brg_support/bot_token",
              value: "",
            },
          ],
        }}
      />
    );

    const presence = screen.getByTestId("bridge-edit-secret-bot_token-presence");
    expect(presence).toHaveTextContent("vault:bridges/brg_support/bot_token");
    expect(screen.queryByLabelText("bot_token")).toBeNull();
    expect(screen.getByTestId("bridge-edit-secret-bot_token-replace")).toHaveTextContent("Rotate");
  });

  it("Should reach the delivery test only from Advanced, with no standalone dialog left", async () => {
    const user = userEvent.setup();
    render(<DialogHarness deliveryTest={deliveryTest} />);

    expect(screen.queryByTestId("bridge-delivery-test-panel")).toBeNull();
    expect(screen.queryByTestId("bridge-test-delivery-dialog")).toBeNull();
    expect(screen.queryByTestId("bridge-send-test-dialog")).toBeNull();

    await user.click(screen.getByTestId("bridge-edit-mode-advanced"));

    expect(screen.getByTestId("bridge-delivery-test-panel")).toBeInTheDocument();
    expect(screen.getByTestId("submit-test-delivery")).toBeInTheDocument();
    expect(screen.getByTestId("submit-send-test")).toBeInTheDocument();
  });

  it("Should run a dry run and a real send from the same target draft", async () => {
    const user = userEvent.setup();
    const onDryRun = vi.fn();
    const onSendTest = vi.fn();
    render(
      <DialogHarness
        deliveryTest={{
          ...deliveryTest,
          draft: { message: "ping", target: { peer_id: "peer_1" } },
          onDryRun,
          onSendTest,
        }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("test-delivery-peer-input")).toHaveValue("peer_1");

    await user.click(screen.getByTestId("submit-test-delivery"));
    await user.click(screen.getByTestId("submit-send-test"));

    expect(onDryRun).toHaveBeenCalledTimes(1);
    expect(onSendTest).toHaveBeenCalledTimes(1);
  });

  it("Should retain one editable delivery target for both the dry run and send", async () => {
    const user = userEvent.setup();
    const onDryRun = vi.fn();
    const onSendTest = vi.fn();

    function DeliveryHarness() {
      const [draft, setDraft] = useState<BridgeTestDeliveryDraft>({ message: "", target: {} });
      return (
        <DialogHarness
          deliveryTest={{ ...deliveryTest, draft, onDraftChange: setDraft, onDryRun, onSendTest }}
          initialMode="advanced"
        />
      );
    }

    render(<DeliveryHarness />);

    await user.type(screen.getByTestId("test-delivery-message"), "operator ping");
    await user.selectOptions(screen.getByTestId("test-delivery-mode-select"), "reply");
    await user.type(screen.getByTestId("test-delivery-peer-input"), "peer_1");
    await user.type(screen.getByTestId("test-delivery-thread-input"), "thread_1");
    await user.type(screen.getByTestId("test-delivery-group-input"), "group_1");

    expect(screen.getByTestId("test-delivery-message")).toHaveValue("operator ping");
    expect(screen.getByTestId("test-delivery-mode-select")).toHaveValue("reply");
    expect(screen.getByTestId("test-delivery-peer-input")).toHaveValue("peer_1");
    expect(screen.getByTestId("test-delivery-thread-input")).toHaveValue("thread_1");
    expect(screen.getByTestId("test-delivery-group-input")).toHaveValue("group_1");

    await user.click(screen.getByTestId("submit-test-delivery"));
    await user.click(screen.getByTestId("submit-send-test"));

    expect(onDryRun).toHaveBeenCalledTimes(1);
    expect(onSendTest).toHaveBeenCalledTimes(1);
  });

  it("Should block a real send while the bridge is disabled and say why", () => {
    render(
      <DialogHarness
        deliveryTest={{
          ...deliveryTest,
          bridgeEnabled: false,
          draft: { message: "ping", target: {} },
        }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("submit-send-test")).toBeDisabled();
    expect(screen.getByTestId("bridge-send-test-disabled-hint")).toHaveTextContent(
      "Enable the bridge to send a real message."
    );
  });

  it("Should require a message before a real send but not before a dry run", () => {
    render(<DialogHarness deliveryTest={deliveryTest} initialMode="advanced" />);

    expect(screen.getByTestId("submit-send-test")).toBeDisabled();
    expect(screen.getByTestId("submit-test-delivery")).toBeEnabled();
  });

  it("Should label each delivery action independently while it is pending", () => {
    const { rerender } = render(
      <DialogHarness
        deliveryTest={{ ...deliveryTest, isDryRunPending: true }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("submit-test-delivery")).toHaveTextContent("Checking…");
    expect(screen.getByTestId("submit-test-delivery")).toBeDisabled();

    rerender(
      <DialogHarness
        deliveryTest={{
          ...deliveryTest,
          draft: { message: "ping", target: {} },
          isSendTestPending: true,
        }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("submit-send-test")).toHaveTextContent("Sending…");
    expect(screen.getByTestId("submit-send-test")).toBeDisabled();
  });

  it("Should render the resolved dry-run target and the delivered send result", () => {
    render(
      <DialogHarness
        deliveryTest={{
          ...deliveryTest,
          dryRunResult: {
            delivery_target: { bridge_instance_id: "brg_support", peer_id: "peer_1" },
            status: "resolved",
          },
          sendTestResult: {
            bridge_instance_id: "brg_support",
            delivery_id: "delivery-001",
            delivery_target: { bridge_instance_id: "brg_support", peer_id: "peer_1" },
            remote_message_id: "remote-9001",
            status: "delivered",
          },
        }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("bridge-test-delivery-result")).toHaveTextContent("resolved");
    expect(screen.getByTestId("bridge-send-test-result")).toHaveTextContent("delivery-001");
    expect(screen.getByTestId("bridge-send-test-result")).toHaveTextContent("remote-9001");
  });

  it("Should warn when the provider accepted the send but returned no verifiable result", () => {
    render(
      <DialogHarness
        deliveryTest={{
          ...deliveryTest,
          sendTestResult: {
            bridge_instance_id: "brg_support",
            delivery_id: "delivery-002",
            delivery_target: { bridge_instance_id: "brg_support", peer_id: "peer_1" },
            error: { message: "Provider receipt is still pending." },
            status: "committed_result_unavailable",
          },
        }}
        initialMode="advanced"
      />
    );

    const result = screen.getByTestId("bridge-send-test-result");
    expect(result).toHaveTextContent("Delivery result is indeterminate");
    expect(result).toHaveTextContent("Provider receipt is still pending.");
    expect(result).not.toHaveTextContent("Remote message ID");
    expect(result).toHaveTextContent("committed_result_unavailable");
  });

  it("Should block submission when provider config JSON is invalid", () => {
    render(
      <DialogHarness
        initialDraft={{ ...baseDraft, providerConfigText: "{invalid" }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("bridge-edit-provider-config-error")).toHaveTextContent(
      "Provider configuration must be valid JSON."
    );
    expect(screen.getByTestId("submit-bridge-edit")).toBeDisabled();
  });

  it("Should show the provider-default option when allowed and a pending submit label", () => {
    render(<DialogHarness allowProviderDefaultDmPolicy isPending />);

    const select = screen.getByTestId("bridge-edit-dm-policy-select");
    const providerDefault = within(select).getByRole("option", { name: "Use provider default" });
    expect(providerDefault).toBeInTheDocument();
    expect(screen.getByTestId("submit-bridge-edit")).toHaveTextContent("Saving…");
  });

  it("Should edit the delivery thread and group targets", async () => {
    const user = userEvent.setup();
    render(<DialogHarness initialMode="advanced" />);

    await user.type(screen.getByTestId("bridge-edit-delivery-thread-input"), "thread_ttt");
    await user.type(screen.getByTestId("bridge-edit-delivery-group-input"), "group_ggg");

    expect(screen.getByTestId("bridge-edit-delivery-thread-input")).toHaveValue("thread_ttt");
    expect(screen.getByTestId("bridge-edit-delivery-group-input")).toHaveValue("group_ggg");
  });

  it("Should update every mutable bridge field through the draft", async () => {
    const user = userEvent.setup();
    render(<DialogHarness initialMode="advanced" />);

    await user.clear(screen.getByTestId("bridge-edit-display-name-input"));
    await user.type(screen.getByTestId("bridge-edit-display-name-input"), "Support Ops");
    await user.selectOptions(screen.getByTestId("bridge-edit-dm-policy-select"), "pairing");
    const routingIncludePeer = screen.getByTestId("bridge-edit-routing-include-peer");
    await user.click(routingIncludePeer);
    await user.selectOptions(screen.getByTestId("bridge-edit-delivery-mode-select"), "reply");
    await user.type(screen.getByTestId("bridge-edit-delivery-peer-input"), "peer_123");

    expect(screen.getByTestId("bridge-edit-display-name-input")).toHaveValue("Support Ops");
    expect(screen.getByTestId("bridge-edit-dm-policy-select")).toHaveValue("pairing");
    expect(routingIncludePeer).toHaveAttribute("aria-checked", "false");
    expect(screen.getByTestId("bridge-edit-delivery-mode-select")).toHaveValue("reply");
    expect(screen.getByTestId("bridge-edit-delivery-peer-input")).toHaveValue("peer_123");
    expect(screen.getByTestId("submit-bridge-edit")).toBeEnabled();
  });

  it("Should write tri-state progress values and remove the override when provider default is restored", async () => {
    const user = userEvent.setup();

    function Wrapper() {
      const [draft, setDraft] = useState<BridgeUpdateDraft>(baseDraft);
      return (
        <>
          <BridgeEditDialog
            allowProviderDefaultDmPolicy={false}
            bridgeName="Support"
            draft={draft}
            identity={identity}
            isPending={false}
            mode="advanced"
            onDraftChange={setDraft}
            onModeChange={() => undefined}
            onOpenChange={vi.fn()}
            onSubmit={vi.fn()}
            open
            pristineDraft={baseDraft}
            provider={makeProvider()}
            rotation={{ kind: "none" }}
          />
          <output data-testid="bridge-edit-progress-draft">
            {draft.deliveryDefaults.progress
              ? JSON.stringify(draft.deliveryDefaults.progress)
              : "provider-default"}
          </output>
        </>
      );
    }

    render(<Wrapper />);

    await user.selectOptions(
      screen.getByTestId("bridge-edit-delivery-progress-mode-select"),
      "verbose"
    );
    const groupingSelect = screen.getByTestId("bridge-edit-delivery-progress-grouping-select");
    expect(within(groupingSelect).queryByRole("option", { name: "Provider default" })).toBeNull();
    expect(groupingSelect.querySelector('option[value=""]')).toBeNull();
    await user.selectOptions(groupingSelect, "separate");
    await user.selectOptions(
      screen.getByTestId("bridge-edit-delivery-progress-typing-select"),
      "true"
    );
    await user.selectOptions(
      screen.getByTestId("bridge-edit-delivery-progress-reactions-select"),
      "false"
    );

    expect(JSON.parse(screen.getByTestId("bridge-edit-progress-draft").textContent ?? "")).toEqual({
      grouping: "separate",
      reactions: false,
      tool_progress: "verbose",
      typing: true,
    });

    await user.selectOptions(screen.getByTestId("bridge-edit-delivery-progress-typing-select"), "");
    expect(JSON.parse(screen.getByTestId("bridge-edit-progress-draft").textContent ?? "")).toEqual({
      grouping: "separate",
      reactions: false,
      tool_progress: "verbose",
    });

    await user.selectOptions(screen.getByTestId("bridge-edit-delivery-progress-mode-select"), "");

    expect(screen.getByTestId("bridge-edit-progress-draft")).toHaveTextContent("provider-default");
    expect(screen.getByTestId("bridge-edit-delivery-progress-grouping-select")).toBeDisabled();
    expect(screen.getByTestId("bridge-edit-delivery-progress-typing-select")).toBeDisabled();
    expect(screen.getByTestId("bridge-edit-delivery-progress-reactions-select")).toBeDisabled();
  });
});
