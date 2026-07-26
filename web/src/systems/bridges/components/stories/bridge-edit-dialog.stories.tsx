import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import type { EntityMode } from "@agh/ui";

import {
  createBridgeUpdateDraft,
  type BridgeDeliveryTestPanelProps,
  type BridgeSecretRotation,
} from "@/systems/bridges";
import { bridgeDetailFixture, bridgeProvidersFixture } from "@/systems/bridges/mocks";
import { BridgeEditDialog } from "@/systems/bridges/components/bridge-edit-dialog";

const bridge = bridgeDetailFixture.bridge;

const identity = {
  extensionName: bridge.extension_name,
  platform: bridge.platform,
  scope: bridge.scope,
  workspaceId: bridge.workspace_id ?? null,
};

const boundRotation: BridgeSecretRotation = {
  editing: false,
  name: bridgeProvidersFixture[0]?.secret_slots?.[0]?.name ?? "bot_token",
  present: true,
  secretRef: `vault:bridges/${bridge.id}/bot_token`,
  value: "",
};

const idleDeliveryTest: Omit<BridgeDeliveryTestPanelProps, "bridgeName"> = {
  bridgeEnabled: bridge.enabled,
  draft: { message: "", target: {} },
  dryRunResult: null,
  isDryRunPending: false,
  isSendTestPending: false,
  onDraftChange: () => undefined,
  onDryRun: () => undefined,
  onSendTest: () => undefined,
  sendTestResult: null,
};

const meta: Meta<typeof BridgeEditDialog> = {
  title: "systems/bridges/components/BridgeEditDialog",
  component: BridgeEditDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Edits the mutable half of a bridge. Platform, extension, and scope are immutable identity; credentials expose presence and rotation only; the delivery test is folded into Advanced as the single entry point (D2).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function BridgeEditDialogHarness({
  initialDraft,
  initialMode = "simple",
  rotations = [boundRotation],
  deliveryTest = idleDeliveryTest,
  hasPendingSecretRotation = false,
}: {
  initialDraft?: ReturnType<typeof createBridgeUpdateDraft>;
  initialMode?: EntityMode;
  rotations?: BridgeSecretRotation[];
  deliveryTest?: typeof idleDeliveryTest;
  hasPendingSecretRotation?: boolean;
}) {
  const pristine = createBridgeUpdateDraft(bridge);
  const [draft, setDraft] = useState(initialDraft ?? pristine);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  const [rotationState, setRotationState] = useState(rotations);

  return (
    <BridgeEditDialog
      allowProviderDefaultDmPolicy={false}
      bridgeName={bridge.display_name}
      deliveryTest={deliveryTest}
      draft={draft}
      hasPendingSecretRotation={hasPendingSecretRotation}
      identity={identity}
      isPending={false}
      mode={mode}
      onDraftChange={setDraft}
      onModeChange={setMode}
      onOpenChange={() => undefined}
      onSubmit={() => undefined}
      open
      pristineDraft={pristine}
      provider={bridgeProvidersFixture[0]}
      rotation={{
        kind: "managed",
        onRotationEditingChange: (name, editing) =>
          setRotationState(current =>
            current.map(rotation => (rotation.name === name ? { ...rotation, editing } : rotation))
          ),
        onRotationValueChange: (name, value) =>
          setRotationState(current =>
            current.map(rotation => (rotation.name === name ? { ...rotation, value } : rotation))
          ),
        rotations: rotationState,
      }}
      statusLabel={`${bridge.enabled ? "enabled" : "disabled"} · ${bridge.platform}`}
    />
  );
}

/** Simple tier: immutable identity plus the mutable name and DM policy. */
export const Default: Story = {
  args: {},
  render: () => <BridgeEditDialogHarness />,
};

/** Advanced tier: rotation, routing, delivery defaults, provider config, delivery test. */
export const Advanced: Story = {
  args: {},
  render: () => <BridgeEditDialogHarness initialMode="advanced" />,
};

/** A credential mid-rotation — the write input is empty and the primary is live. */
export const SecretRotating: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      hasPendingSecretRotation
      initialMode="advanced"
      rotations={[{ ...boundRotation, editing: true, value: "xoxb-new" }]}
    />
  ),
};

/** A rotation that failed: the reason shows, the typed value does not survive. */
export const SecretRotationError: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      initialMode="advanced"
      rotations={[
        { ...boundRotation, editing: true, error: "Vault rejected the value.", value: "" },
      ]}
    />
  ),
};

/** A completed rotation, confirmed without echoing the new value. */
export const SecretRotated: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      initialMode="advanced"
      rotations={[{ ...boundRotation, rotated: true }]}
    />
  ),
};

/** The dry run resolved a target and the real send delivered. */
export const DeliveryTestResults: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      deliveryTest={{
        ...idleDeliveryTest,
        draft: { message: "Operator ping", target: { peer_id: "peer_1" } },
        dryRunResult: {
          delivery_target: { bridge_instance_id: bridge.id, peer_id: "peer_1" },
          status: "resolved",
        },
        sendTestResult: {
          bridge_instance_id: bridge.id,
          delivery_id: "delivery-001",
          delivery_target: { bridge_instance_id: bridge.id, peer_id: "peer_1" },
          remote_message_id: "remote-9001",
          status: "delivered",
        },
      }}
      initialMode="advanced"
    />
  ),
};

/** The provider accepted the send but returned no verifiable result. */
export const DeliveryTestFailure: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      deliveryTest={{
        ...idleDeliveryTest,
        draft: { message: "Operator ping", target: { peer_id: "peer_1" } },
        sendTestResult: {
          bridge_instance_id: bridge.id,
          delivery_id: "delivery-002",
          delivery_target: { bridge_instance_id: bridge.id, peer_id: "peer_1" },
          error: { message: "Provider returned 502 before acknowledging the message" },
          status: "committed_result_unavailable",
        },
      }}
      initialMode="advanced"
    />
  ),
};

/** A disabled bridge can still resolve a target but cannot send a real message. */
export const DeliveryTestDisabledBridge: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      deliveryTest={{
        ...idleDeliveryTest,
        bridgeEnabled: false,
        draft: { message: "Operator ping", target: {} },
      }}
      initialMode="advanced"
    />
  ),
};

/** Provider configuration validation without hiding the rest of the editor. */
export const InvalidProviderConfig: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      initialDraft={{
        ...createBridgeUpdateDraft(bridge),
        providerConfigText: "{ invalid json",
      }}
      initialMode="advanced"
    />
  ),
};

/** An explicit progress override hydrated from persisted delivery defaults. */
export const ProgressOverride: Story = {
  args: {},
  render: () => (
    <BridgeEditDialogHarness
      initialDraft={{
        ...createBridgeUpdateDraft(bridge),
        deliveryDefaults: {
          ...createBridgeUpdateDraft(bridge).deliveryDefaults,
          progress: {
            grouping: "separate",
            reactions: false,
            tool_progress: "verbose",
            typing: true,
          },
        },
      }}
      initialMode="advanced"
    />
  ),
};
