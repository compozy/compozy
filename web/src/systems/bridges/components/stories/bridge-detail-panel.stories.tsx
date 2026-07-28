import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { storyDefaultWorkspaceName } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import { projectBridgeSetup, type BridgeSetupProjection } from "@/systems/bridges/lib/bridge-setup";
import {
  bridgeDetailFixture,
  bridgeProvidersFixture,
  bridgeResolveTargetFixture,
  bridgeRoutesFixture,
  bridgeSecretBindingsFixture,
  bridgeTargetsFixture,
  bridgeVerifyFixture,
} from "@/systems/bridges/mocks";
import type {
  BridgeHealth,
  BridgeProvider,
  BridgeSummary,
  BridgeVerifyResponse,
} from "@/systems/bridges/types";

import { BridgeDetailPanel } from "../bridge-detail-panel";

const meta: Meta<typeof BridgeDetailPanel> = {
  component: BridgeDetailPanel,
  parameters: { layout: "fullscreen" },
  title: "systems/bridges/components/BridgeDetailPanel",
};

export default meta;
type Story = StoryObj<typeof meta>;

function requireSlackProvider(): BridgeProvider {
  const provider = bridgeProvidersFixture.find(candidate => candidate.platform === "slack");
  if (!provider) throw new globalThis.Error("Slack story fixture is missing its provider");
  return provider;
}

const slackProvider = requireSlackProvider();
const slackBridge = bridgeDetailFixture.bridge;
const slackHealth = bridgeDetailFixture.health;
const fullyConfiguredProjection = projectBridgeSetup({
  bindings: bridgeSecretBindingsFixture,
  bridge: slackBridge,
  health: slackHealth,
  provider: slackProvider,
  registration: null,
  verification: bridgeVerifyFixture,
});

const failedSlackBridge: BridgeSummary = {
  ...slackBridge,
  enabled: false,
  status: "disabled",
};
const failedSlackHealth: BridgeHealth = {
  ...slackHealth,
  status: "disabled",
};
const failedSlackVerification: BridgeVerifyResponse = {
  bridge_instance_id: failedSlackBridge.id,
  checks: [
    {
      check: "provider.identity",
      remediation: "Replace the rejected Slack bot token, then verify again.",
      status: "fail",
    },
    { check: "webhook.signing_secret", remediation: "", status: "pass" },
    {
      check: "webhook.reachability",
      remediation: "Enable the bridge, then run verification again.",
      status: "skipped",
    },
  ],
};
const failedVerifyProjection = projectBridgeSetup({
  bindings: bridgeSecretBindingsFixture,
  bridge: failedSlackBridge,
  health: failedSlackHealth,
  provider: slackProvider,
  registration: null,
  verification: failedSlackVerification,
});

function setup(projection: BridgeSetupProjection) {
  return {
    isLifecyclePending: false,
    isRegistering: false,
    isVerifying: false,
    onRegisterWebhook: () => undefined,
    onVerify: () => undefined,
    projection,
  };
}

function targetDirectory() {
  return {
    error: null,
    isLoading: false,
    isResolving: false,
    onQueryChange: () => undefined,
    onResolveInputChange: () => undefined,
    onResolveSubmit: () => undefined,
    query: "",
    resolveInput: "Launch room",
    resolveResult: bridgeResolveTargetFixture,
    response: bridgeTargetsFixture,
  };
}

export const FullyConfigured: Story = {
  render: () => (
    <PanelSurface>
      <BridgeDetailPanel
        bridge={slackBridge}
        error={null}
        health={slackHealth}
        onOpenDeliveryTest={() => undefined}
        provider={slackProvider}
        routes={bridgeRoutesFixture}
        secretBindings={bridgeSecretBindingsFixture}
        setup={setup(fullyConfiguredProjection)}
        state={{ isLoading: false, isRoutesLoading: false }}
        targetDirectory={targetDirectory()}
        workspaceName={storyDefaultWorkspaceName}
      />
    </PanelSurface>
  ),
};

export const FailedVerify: Story = {
  render: () => (
    <PanelSurface>
      <BridgeDetailPanel
        bridge={failedSlackBridge}
        error={null}
        health={failedSlackHealth}
        onOpenDeliveryTest={() => undefined}
        provider={slackProvider}
        routes={[]}
        secretBindings={bridgeSecretBindingsFixture}
        setup={setup(failedVerifyProjection)}
        state={{ isLoading: false, isRoutesLoading: false }}
        targetDirectory={targetDirectory()}
        workspaceName={storyDefaultWorkspaceName}
      />
    </PanelSurface>
  ),
};

export const BindingsUnavailable: Story = {
  tags: ["play-fn"],
  render: () => (
    <PanelSurface>
      <BridgeDetailPanel
        bridge={slackBridge}
        error={null}
        health={slackHealth}
        onOpenDeliveryTest={() => undefined}
        provider={slackProvider}
        routes={bridgeRoutesFixture}
        secretBindings={bridgeSecretBindingsFixture}
        setup={setup(fullyConfiguredProjection)}
        state={{
          isLoading: false,
          isRoutesLoading: false,
          secretBindingsError: new globalThis.Error("Vault bindings could not be loaded."),
        }}
        workspaceName={storyDefaultWorkspaceName}
      />
    </PanelSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const unavailable = await canvas.findByTestId("bridge-secret-bindings-unavailable");
    unavailable.scrollIntoView({ block: "center" });
    await expect(unavailable).toBeInTheDocument();
  },
};

export const NoRoutes: Story = {
  render: () => (
    <PanelSurface>
      <BridgeDetailPanel
        bridge={slackBridge}
        error={null}
        health={slackHealth}
        onOpenDeliveryTest={() => undefined}
        provider={slackProvider}
        routes={[]}
        secretBindings={bridgeSecretBindingsFixture}
        setup={setup(fullyConfiguredProjection)}
        state={{ isLoading: false, isRoutesLoading: false }}
        workspaceName={storyDefaultWorkspaceName}
      />
    </PanelSurface>
  ),
};

export const Error: Story = {
  render: () => (
    <PanelSurface>
      <BridgeDetailPanel
        bridge={undefined}
        error={new globalThis.Error("Failed to load bridge details")}
        health={undefined}
        onOpenDeliveryTest={() => undefined}
        routes={[]}
        setup={setup(fullyConfiguredProjection)}
        state={{ isLoading: false, isRoutesLoading: false }}
      />
    </PanelSurface>
  ),
};
