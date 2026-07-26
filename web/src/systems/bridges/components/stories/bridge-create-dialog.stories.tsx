import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import type { EntityMode } from "@agh/ui";

import { storyDefaultWorkspaceId, storyDefaultWorkspaceName } from "@/storybook/fintech-scenario";
import { createBridgeCreateDraft, type BridgeSecretRecoveryState } from "@/systems/bridges";
import { bridgeProvidersFixture, slackBridgeManifestFixture } from "@/systems/bridges/mocks";

import { BridgeCreateDialog } from "../bridge-create-dialog";
import type { BridgeManifestCommittedState } from "../bridge-manifest-handoff";

const manifestJSON = JSON.stringify(slackBridgeManifestFixture.manifest, null, 2);

const meta: Meta<typeof BridgeCreateDialog> = {
  title: "systems/bridges/components/BridgeCreateDialog",
  component: BridgeCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Creates a bridge on the shared entity-dialog shell. Simple carries the provider, identity, and the provider-declared credential slots; Advanced adds routing, delivery, and provider config. Secrets are bound after the bridge exists — they never enter the create body.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function BridgeCreateDialogHarness({
  initialMode = "simple",
  manifestState,
  secretRecovery,
  supportsManifest = false,
  providers = bridgeProvidersFixture,
}: {
  initialMode?: EntityMode;
  manifestState?: BridgeManifestCommittedState;
  secretRecovery?: BridgeSecretRecoveryState | null;
  supportsManifest?: boolean;
  providers?: typeof bridgeProvidersFixture;
}) {
  const [draft, setDraft] = useState(createBridgeCreateDraft(providers, storyDefaultWorkspaceId));
  const [mode, setMode] = useState<EntityMode>(initialMode);

  return (
    <BridgeCreateDialog
      activeWorkspaceId={storyDefaultWorkspaceId}
      activeWorkspaceName={storyDefaultWorkspaceName}
      draft={draft}
      isPending={false}
      manifestState={manifestState}
      mode={mode}
      onDraftChange={setDraft}
      onModeChange={setMode}
      onOpenChange={() => undefined}
      onSubmit={() => undefined}
      open
      providers={providers}
      secretRecovery={secretRecovery ?? null}
      supportsManifest={supportsManifest}
    />
  );
}

/** Simple tier: provider choice, identity, and the declared credential slots. */
export const Default: Story = {
  args: {},
  render: () => <BridgeCreateDialogHarness />,
};

/** Advanced tier: DM policy, routing, delivery defaults, and provider config. */
export const Advanced: Story = {
  args: {},
  render: () => <BridgeCreateDialogHarness initialMode="advanced" />,
};

/** A provider that declares no credential slots at all. */
export const NoSecretSlots: Story = {
  args: {},
  render: () => (
    <BridgeCreateDialogHarness
      providers={bridgeProvidersFixture.map(provider => ({ ...provider, secret_slots: [] }))}
    />
  ),
};

/** Required and optional slots side by side, so the gating is visible. */
export const MixedSlotRequirements: Story = {
  args: {},
  render: () => (
    <BridgeCreateDialogHarness
      providers={bridgeProvidersFixture.map(provider => ({
        ...provider,
        secret_slots: [
          { description: "Bot token", name: "bot_token", required: true },
          { description: "Socket-mode app token", name: "app_token", required: false },
        ],
      }))}
    />
  ),
};

/** Switching provider clears every slot draft — a typed value never travels. */
export const ProviderSwitchClearsSlots: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => <BridgeCreateDialogHarness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const radios = await canvas.findAllByRole("radio");
    await userEvent.click(radios[radios.length - 1]);
    await expect(canvas.getByTestId("bridge-create-section-credentials")).toBeInTheDocument();
  },
};

/** Phase two failed for one slot: the bridge exists, the plaintext does not. */
export const SecretBindingRecovery: Story = {
  args: {},
  render: () => (
    <BridgeCreateDialogHarness
      secretRecovery={{
        bridgeId: "brg_launch_room",
        bound: ["signing_secret"],
        failures: { bot_token: "Vault is unavailable. The value was not stored." },
        provider: bridgeProvidersFixture[0],
      }}
    />
  ),
};

/** Manifest providers issue credentials later, so slots never gate the create. */
export const SlackDeferredCredentials: Story = {
  args: {},
  render: () => <BridgeCreateDialogHarness supportsManifest />,
};

/** Shows the deterministic post-create Slack manifest handoff. */
export const SlackManifestHandoff: Story = {
  args: {},
  render: () => (
    <BridgeCreateDialogHarness
      manifestState={{
        bridgeId: "brg_launch_room",
        isLoading: false,
        manifestJSON,
        onOpenBridge: () => undefined,
        onRetry: () => undefined,
      }}
      supportsManifest
    />
  ),
};

/** Shows recovery after the bridge persisted but manifest retrieval failed. */
export const SlackManifestError: Story = {
  args: {},
  render: () => (
    <BridgeCreateDialogHarness
      manifestState={{
        bridgeId: "brg_launch_room",
        error: "The saved webhook URL is not valid for a Slack manifest.",
        isLoading: false,
        onOpenBridge: () => undefined,
        onRetry: () => undefined,
      }}
      supportsManifest
    />
  ),
};
