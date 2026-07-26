import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { EntityMode } from "@agh/ui";

import { BridgeCreateDialog } from "@/systems/bridges/components/bridge-create-dialog";
import type { BridgeCreateDraft, BridgeProvider } from "@/systems/bridges/types";

const baseDraft: BridgeCreateDraft = {
  deliveryDefaults: {},
  dmPolicy: "",
  displayName: "",
  enabled: false,
  notificationSuppress: false,
  secretSlotValues: {},
  providerConfigText: "",
  routingPolicy: { include_group: true, include_peer: true, include_thread: true },
  scope: "global",
  selectedProviderKey: "",
};

function makeProvider(overrides: Partial<BridgeProvider> = {}): BridgeProvider {
  return {
    config_schema: {
      schema: "provider-config",
      version: "2026-04-15",
    },
    description: "Provider-specific runtime settings",
    display_name: "Telegram",
    enabled: true,
    extension_name: "ext-telegram",
    health: "healthy",
    platform: "telegram",
    secret_slots: [
      {
        description: "Bot API token",
        name: "bot_token",
        required: true,
      },
    ],
    state: "active",
    ...overrides,
  };
}

function createDeferredRequest() {
  let resolve!: () => void;
  const promise = new Promise<void>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

/** Stateful host so mode and draft transitions behave like the real flow hook. */
function DialogHarness({
  initialDraft = baseDraft,
  initialMode = "simple",
  providers = [makeProvider()],
  ...overrides
}: Partial<Parameters<typeof BridgeCreateDialog>[0]> & {
  initialDraft?: BridgeCreateDraft;
  initialMode?: EntityMode;
}) {
  const [draft, setDraft] = useState<BridgeCreateDraft>(initialDraft);
  const [mode, setMode] = useState<EntityMode>(initialMode);
  return (
    <>
      <BridgeCreateDialog
        activeWorkspaceId="ws_test"
        activeWorkspaceName="test-workspace"
        draft={draft}
        isPending={false}
        mode={mode}
        onDraftChange={setDraft}
        onModeChange={setMode}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
        open
        providers={providers}
        {...overrides}
      />
      <output data-testid="bridge-create-draft-probe">{JSON.stringify(draft)}</output>
    </>
  );
}

function readDraftProbe(): BridgeCreateDraft {
  return JSON.parse(screen.getByTestId("bridge-create-draft-probe").textContent ?? "{}");
}

describe("BridgeCreateDialog", () => {
  it("Should present a single-surface editor with no wizard chrome left", () => {
    render(<DialogHarness providers={[]} />);

    expect(screen.getByRole("dialog", { name: "Create bridge" })).toBeInTheDocument();
    expect(screen.queryByTestId("bridge-wizard-stepper")).toBeNull();
    expect(screen.queryByTestId("bridge-wizard-progress")).toBeNull();
    expect(screen.queryByTestId("bridge-wizard-next")).toBeNull();
    expect(screen.queryByTestId("bridge-wizard-back")).toBeNull();
  });

  it("Should render the provider empty state and gate submit when no provider is available", () => {
    render(<DialogHarness providers={[]} />);

    expect(screen.getByTestId("bridge-provider-empty")).toHaveTextContent(
      "No bridge providers are currently available."
    );
    expect(screen.getByTestId("submit-bridge-create")).toBeDisabled();
  });

  it("Should render one write-only SecretField per provider-declared slot", () => {
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Slack",
          selectedProviderKey: "ext-slack::slack",
        }}
        providers={[
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
            secret_slots: [
              { description: "Bot token", name: "bot_token", required: true },
              { description: "Signing secret", name: "signing_secret", required: true },
              { description: "Socket app token", name: "app_token", required: false },
            ],
          }),
        ]}
      />
    );

    const credentials = screen.getByTestId("bridge-create-section-credentials");
    expect(within(credentials).getByText("bot_token")).toBeInTheDocument();
    expect(within(credentials).getByText("signing_secret")).toBeInTheDocument();
    expect(within(credentials).getByText("app_token")).toBeInTheDocument();
  });

  it("Should keep submit gated until every required slot carries a value", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
      />
    );

    expect(screen.getByTestId("submit-bridge-create")).toBeDisabled();

    await user.type(screen.getByLabelText("bot_token"), "123:abc");

    expect(screen.getByTestId("submit-bridge-create")).toBeEnabled();
  });

  it("Should never carry a typed slot value across a provider switch", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
        providers={[
          makeProvider(),
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
            secret_slots: [{ description: "Signing secret", name: "signing_secret" }],
          }),
        ]}
      />
    );

    await user.type(screen.getByLabelText("bot_token"), "123:abc");
    expect(readDraftProbe().secretSlotValues.bot_token).toBe("123:abc");

    await user.click(screen.getByRole("radio", { name: /Slack/ }));

    expect(readDraftProbe().secretSlotValues).toEqual({});
    expect(screen.queryByLabelText("bot_token")).toBeNull();
    expect(screen.getByLabelText("signing_secret")).toHaveValue("");
  });

  it("Should keep routing, delivery and provider config behind Advanced", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
      />
    );

    expect(screen.queryByTestId("bridge-provider-config-input")).toBeNull();
    expect(screen.queryByTestId("bridge-dm-policy-select")).toBeNull();
    // Credentials and identity are never hidden by the disclosure tier.
    expect(screen.getByTestId("bridge-display-name-input")).toBeInTheDocument();

    await user.click(screen.getByTestId("bridge-create-mode-advanced"));

    expect(screen.getByTestId("bridge-provider-config-input")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-dm-policy-select")).toBeInTheDocument();
    expect(screen.getByTestId("bridge-display-name-input")).toBeInTheDocument();
  });

  it("Should block submit while provider config is invalid JSON", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          providerConfigText: "{invalid",
          secretSlotValues: { bot_token: "123:abc" },
          selectedProviderKey: "ext-telegram::telegram",
        }}
        initialMode="advanced"
      />
    );

    expect(screen.getByTestId("bridge-provider-config-error")).toHaveTextContent(
      "Provider configuration must be valid JSON."
    );
    expect(screen.getByTestId("submit-bridge-create")).toBeDisabled();

    await user.clear(screen.getByTestId("bridge-provider-config-input"));

    expect(screen.getByTestId("submit-bridge-create")).toBeEnabled();
  });

  it("Should never send a typed secret through provider_config", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
      />
    );

    await user.type(screen.getByLabelText("bot_token"), "123:abc");

    const draft = readDraftProbe();
    expect(draft.providerConfigText).toBe("");
    expect(draft.secretSlotValues.bot_token).toBe("123:abc");
  });

  it("Should offer a recovery retry that never re-displays the earlier plaintext", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Slack",
          selectedProviderKey: "ext-slack::slack",
        }}
        onSubmit={onSubmit}
        providers={[
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
            secret_slots: [
              { description: "Bot token", name: "bot_token", required: true },
              { description: "Signing secret", name: "signing_secret", required: true },
            ],
          }),
        ]}
        secretRecovery={{
          bridgeId: "brg_slack",
          bound: ["signing_secret"],
          failures: { bot_token: "vault unavailable" },
          provider: makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
            secret_slots: [
              { description: "Bot token", name: "bot_token", required: true },
              { description: "Signing secret", name: "signing_secret", required: true },
            ],
          }),
        }}
      />
    );

    // The alert names what still has to be bound; the identity block names the
    // bridge that already exists and can no longer be cancelled away.
    expect(screen.getByTestId("bridge-create-secret-recovery")).toHaveTextContent("bot_token");
    expect(screen.getByText("brg_slack")).toBeInTheDocument();
    expect(screen.getByLabelText("bot_token")).toHaveValue("");
    expect(screen.getByTestId("submit-bridge-create")).toHaveTextContent("Retry secret binding");
    expect(screen.getByTestId("submit-bridge-create")).toBeDisabled();

    await user.type(screen.getByLabelText("bot_token"), "new-token");
    await user.click(screen.getByTestId("submit-bridge-create"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should keep provider identity immutable while creation is pending", async () => {
    const user = userEvent.setup();
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
        isPending
        providers={[
          makeProvider(),
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
          }),
        ]}
      />
    );

    const slack = screen.getByRole("radio", { name: /Slack/ });
    expect(slack).toHaveAttribute("tabindex", "-1");
    await user.click(slack);

    expect(readDraftProbe().selectedProviderKey).toBe("ext-telegram::telegram");
  });

  it("Should remove activation from the creation surface until setup is complete", () => {
    render(<DialogHarness />);

    expect(screen.queryByTestId("bridge-create-enabled")).toBeNull();
  });

  it("Should block Cancel and dialog dismissal while the create request is pending", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const onSubmit = vi.fn();
    const request = createDeferredRequest();

    function Wrapper() {
      const [draft, setDraft] = useState<BridgeCreateDraft>({
        ...baseDraft,
        displayName: "Telegram",
        secretSlotValues: { bot_token: "123:abc" },
        selectedProviderKey: "ext-telegram::telegram",
      });
      const [isPending, setIsPending] = useState(false);
      const [open, setOpen] = useState(true);

      const handleSubmit = async () => {
        onSubmit();
        setIsPending(true);
        await request.promise;
        setIsPending(false);
      };

      return (
        <BridgeCreateDialog
          activeWorkspaceId="ws_test"
          activeWorkspaceName="test-workspace"
          draft={draft}
          isPending={isPending}
          onDraftChange={setDraft}
          onOpenChange={next => {
            onOpenChange(next);
            setOpen(next);
          }}
          onSubmit={handleSubmit}
          open={open}
          providers={[makeProvider()]}
        />
      );
    }

    render(<Wrapper />);

    await user.click(screen.getByTestId("submit-bridge-create"));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("bridge-wizard-cancel")).toBeDisabled();

    await user.keyboard("{Escape}");
    expect(screen.getByTestId("bridge-create-dialog")).toBeInTheDocument();
    expect(onOpenChange).not.toHaveBeenCalled();

    await act(async () => {
      request.resolve();
      await request.promise;
    });
    await waitFor(() => expect(screen.getByTestId("bridge-wizard-cancel")).toBeEnabled());

    await user.click(screen.getByTestId("bridge-wizard-cancel"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should write a complete progress override and remove it when provider default is restored", async () => {
    const user = userEvent.setup();

    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Telegram",
          selectedProviderKey: "ext-telegram::telegram",
        }}
        initialMode="advanced"
      />
    );

    await user.selectOptions(screen.getByTestId("bridge-delivery-progress-mode-select"), "all");
    const groupingSelect = screen.getByTestId("bridge-delivery-progress-grouping-select");
    expect(within(groupingSelect).queryByRole("option", { name: "Provider default" })).toBeNull();
    expect(groupingSelect.querySelector('option[value=""]')).toBeNull();
    await user.selectOptions(groupingSelect, "separate");
    await user.selectOptions(screen.getByTestId("bridge-delivery-progress-typing-select"), "true");
    await user.selectOptions(
      screen.getByTestId("bridge-delivery-progress-reactions-select"),
      "false"
    );

    expect(readDraftProbe().deliveryDefaults.progress).toEqual({
      grouping: "separate",
      reactions: false,
      tool_progress: "all",
      typing: true,
    });

    await user.selectOptions(screen.getByTestId("bridge-delivery-progress-mode-select"), "");

    expect(readDraftProbe().deliveryDefaults.progress).toBeUndefined();
    expect(screen.getByTestId("bridge-delivery-progress-grouping-select")).toBeDisabled();
    expect(screen.getByTestId("bridge-delivery-progress-typing-select")).toBeDisabled();
    expect(screen.getByTestId("bridge-delivery-progress-reactions-select")).toBeDisabled();
  });

  it("Should advertise a post-create manifest only when the selected provider supports it", () => {
    const slackProvider = makeProvider({
      display_name: "Slack",
      extension_name: "ext-slack",
      platform: "slack",
    });
    const props = {
      activeWorkspaceId: "ws_test",
      activeWorkspaceName: "test-workspace",
      draft: {
        ...baseDraft,
        displayName: "Slack",
        selectedProviderKey: "ext-slack::slack",
      },
      isPending: false,
      onDraftChange: vi.fn(),
      onOpenChange: vi.fn(),
      onSubmit: vi.fn(),
      open: true,
      providers: [slackProvider],
    };

    const { rerender } = render(<BridgeCreateDialog {...props} supportsManifest />);

    expect(screen.getByTestId("bridge-manifest-precreate-hint")).toHaveTextContent(
      "Slack manifest available after creation"
    );
    expect(screen.queryByTestId("bridge-manifest-json")).not.toBeInTheDocument();

    rerender(
      <BridgeCreateDialog
        {...props}
        manifestState={{
          bridgeId: "brg_slack",
          isLoading: false,
          manifestJSON: '{"name":"Slack"}',
          onOpenBridge: vi.fn(),
          onRetry: vi.fn(),
        }}
        supportsManifest={false}
      />
    );

    expect(screen.queryByTestId("bridge-manifest-precreate-hint")).not.toBeInTheDocument();
    expect(screen.queryByTestId("bridge-manifest-handoff")).not.toBeInTheDocument();
  });

  it("Should defer credential entry for a manifest provider instead of gating create on it", () => {
    render(
      <DialogHarness
        initialDraft={{
          ...baseDraft,
          displayName: "Slack",
          selectedProviderKey: "ext-slack::slack",
        }}
        providers={[
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
          }),
        ]}
        supportsManifest
      />
    );

    expect(screen.getByTestId("bridge-create-secret-deferred")).toHaveTextContent(
      "bind them from the bridge detail"
    );
    expect(screen.getByTestId("submit-bridge-create")).toBeEnabled();
  });

  it("Should render the committed Slack manifest with copy and dashboard handoff", async () => {
    const user = userEvent.setup();
    const manifestJSON = JSON.stringify({ display_information: { name: "AGH Support" } }, null, 2);
    const onOpenBridge = vi.fn();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(
      <BridgeCreateDialog
        draft={{
          ...baseDraft,
          displayName: "Slack",
          selectedProviderKey: "ext-slack::slack",
        }}
        isPending={false}
        manifestState={{
          bridgeId: "brg_slack",
          isLoading: false,
          manifestJSON,
          onOpenBridge,
          onRetry: vi.fn(),
        }}
        onDraftChange={vi.fn()}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
        open
        providers={[
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
          }),
        ]}
        supportsManifest
      />
    );

    expect(screen.getByRole("heading", { name: "Set up Slack app" })).toBeInTheDocument();
    expect(screen.getByTestId("bridge-manifest-json")).toHaveTextContent("AGH Support");
    expect(screen.getByRole("link", { name: "Open Slack app dashboard" })).toHaveAttribute(
      "href",
      "https://api.slack.com/apps"
    );
    expect(screen.queryByTestId("submit-bridge-create")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Copy Slack app manifest" }));
    expect(writeText).toHaveBeenCalledWith(manifestJSON);

    await user.click(screen.getByTestId("bridge-manifest-open-bridge"));
    expect(onOpenBridge).toHaveBeenCalledTimes(1);
  });

  it("Should preserve recovery actions when the committed manifest fails", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    const onOpenBridge = vi.fn();

    render(
      <BridgeCreateDialog
        draft={{
          ...baseDraft,
          displayName: "Slack",
          selectedProviderKey: "ext-slack::slack",
        }}
        isPending={false}
        manifestState={{
          bridgeId: "brg_slack",
          error: "Saved webhook URL is invalid.",
          isLoading: false,
          onOpenBridge,
          onRetry,
        }}
        onDraftChange={vi.fn()}
        onOpenChange={vi.fn()}
        onSubmit={vi.fn()}
        open
        providers={[
          makeProvider({
            display_name: "Slack",
            extension_name: "ext-slack",
            platform: "slack",
          }),
        ]}
        supportsManifest
      />
    );

    expect(screen.getByTestId("bridge-manifest-error")).toHaveTextContent(
      "Saved webhook URL is invalid."
    );
    expect(screen.queryByTestId("bridge-manifest-json")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Open Slack app dashboard" })
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Retry" }));
    await user.click(screen.getByTestId("bridge-manifest-open-bridge"));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(onOpenBridge).toHaveBeenCalledTimes(1);
  });
});
