import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { agentFixtures } from "@/systems/agent/mocks";
import type { NetworkParticipationDraft } from "@/systems/network";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";

import { SessionCreateDialog } from "../session-create-dialog";

const workspace = workspaceDetailFixture.workspace;

// Truthful post-migration args: the dialog now hosts one unified RuntimeSelector
// (provider · model · reasoning) fed by aggregate catalog rows, not the deleted
// provider/model/reasoning leaf selects.
const runtimeProviders: RuntimeProviderOption[] = [
  { id: "codex", name: "Codex", runtime_provider: "codex", harness: "acp" },
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
];

const runtimeModels: RuntimeModelOption[] = [
  {
    id: "gpt-5.6-sol",
    provider: "codex",
    name: "GPT-5.6 Sol",
    context_window: 1_050_000,
    cost_input: 5,
    cost_output: 30,
    supports_tools: true,
    supports_reasoning: true,
    efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_effort: "",
    reasoning_source: "acp",
    availability: "live",
    curated: true,
    featured: true,
  },
  {
    id: "claude-fable-5",
    provider: "claude",
    name: "Claude Fable 5",
    context_window: 1_000_000,
    cost_input: 10,
    cost_output: 50,
    supports_tools: true,
    supports_reasoning: true,
    efforts: ["low", "medium", "high", "xhigh", "max"],
    default_effort: "",
    reasoning_source: "acp",
    availability: "live",
    curated: true,
    featured: true,
  },
];

const runtimeValue = {
  provider: "codex",
  model: "gpt-5.6-sol",
  reasoning_effort: "high",
} satisfies RuntimeSelectorValue;

const FIRST_MESSAGE =
  "Audit the release notes for 0.9 and flag anything that claims behaviour we have not shipped.";

const localParticipation = {
  mode: "local",
  channelStrategy: "",
  channelId: "",
} satisfies NetworkParticipationDraft;
const liveParticipation = {
  mode: "live",
  channelStrategy: "named",
  channelId: "release-room",
} satisfies NetworkParticipationDraft;

const baseArgs = {
  open: true,
  onOpenChange: fn(),
  mode: "simple" as const,
  onModeChange: fn(),
  agents: agentFixtures,
  workspace,
  workspaces: [{ id: workspace.id, name: workspace.name, root_dir: workspace.root_dir }],
  workspaceId: workspace.id,
  onWorkspaceChange: fn(),
  sessionName: "Investigate checkout latency",
  onSessionNameChange: fn(),
  workspacePath: "",
  onWorkspacePathChange: fn(),
  selectedAgentName: agentFixtures[0]?.name ?? "",
  runtimeValue,
  runtimeProviders,
  runtimeModels,
  catalogStale: false,
  catalogLoading: false,
  catalogLoaded: true,
  catalogError: null,
  catalogRefreshing: false,
  catalogRefreshError: null,
  providersLoading: false,
  providersError: null,
  hasProviderOptions: true,
  networkParticipation: localParticipation,
  promptValue: FIRST_MESSAGE,
  onPromptChange: fn(),
  onAgentChange: fn(),
  onRuntimeChange: fn(),
  onNetworkParticipationChange: fn(),
  onCatalogRefresh: fn(),
  onOpenProviderSettings: fn(),
  onSubmit: fn(),
  isSubmitting: false,
  submitError: null,
};

const meta: Meta<typeof SessionCreateDialog> = {
  title: "systems/session/components/SessionCreateDialog",
  component: SessionCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Session-create dialog reshaped as a first-message composer: agent + Network above, then the prompt box that owns the chromeless RuntimeSelector (lower-left) and the accent Send disc (lower-right) — the only creation action. Enter sends, Shift+Enter inserts a newline.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * VC-01 capture target: Simple fields plus a drafted first message. The
 * chromeless runtime trigger sits inside the prompt box and Send is armed.
 */
export const Default: Story = {
  args: baseArgs,
};

/**
 * Empty draft: Send is disabled because a session is never created without a
 * first message.
 */
export const EmptyDraft: Story = {
  args: {
    ...baseArgs,
    promptValue: "",
  },
};

/**
 * No active workspace: the composer is inert end to end — dimmed box, dead
 * textarea, unpressable runtime trigger and Send disc — and the header states
 * the one thing that unblocks it.
 */
export const NoWorkspace: Story = {
  args: {
    ...baseArgs,
    workspace: undefined,
    promptValue: "",
  },
};

/**
 * Runtime selector open over the composer — the trigger must stay borderless
 * and unfilled while the popup is up.
 */
export const RuntimeSelectorOpen: Story = {
  args: baseArgs,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    // Dialog and popup are both portaled, so query from the document body.
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("session-create-runtime-select"));
    await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  },
};

/**
 * VC-02 capture target: Advanced tier — participation and working path are
 * revealed without hiding the Simple fields or composer-owned runtime control.
 */
export const Advanced: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    workspacePath: "services/checkout",
  },
};

/**
 * VC-09 capture target: ≤760px full-width bottom surface with 44px targets.
 */
export const Mobile: Story = {
  args: baseArgs,
  parameters: {
    viewport: { defaultViewport: "iphone14" },
  },
};

/**
 * Catalog state lives inside the RuntimeSelector popover, never in the dialog
 * chrome, so these stories open the popover to make the status line visible.
 */
async function openRuntimePopup(canvasElement: HTMLElement) {
  const body = within(canvasElement.ownerDocument.body);
  await userEvent.click(await body.findByTestId("session-create-runtime-select"));
  await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  return body;
}

/** Stale catalog: the status line warns without blocking submit. */
export const CatalogStale: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    catalogStale: true,
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = await openRuntimePopup(canvasElement);
    await expect(await body.findByTestId("session-create-catalog-stale")).toBeVisible();
  },
};

/** Catalog source failure — the draft survives and only the model picker degrades. */
export const CatalogError: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    runtimeModels: [],
    catalogError: "Model catalog request failed.",
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = await openRuntimePopup(canvasElement);
    await expect(await body.findByTestId("session-create-catalog-error")).toBeVisible();
  },
};

/** Empty catalog — the provider default remains selectable. */
export const CatalogEmpty: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    runtimeModels: [],
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = await openRuntimePopup(canvasElement);
    await expect(await body.findByTestId("session-create-catalog-empty")).toBeVisible();
  },
};

/**
 * Keyboard traversal reaches the header close control with a visible 2px ring.
 * Focus must arrive through real tabbing — a programmatic `.focus()` sets
 * `:focus` without `:focus-visible`, so the ring would never render.
 */
export const FocusVisibleClose: Story = {
  args: baseArgs,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const close = await body.findByRole("button", { name: "Close" });
    const doc = canvasElement.ownerDocument;
    for (let step = 0; step < 12 && doc.activeElement !== close; step += 1) {
      await userEvent.tab();
    }
    await expect(close).toHaveFocus();
    // The ring itself is a `:focus-visible` pseudo-class, not a DOM class, so it
    // is verified visually in the capture rather than asserted here.
  },
};

/**
 * Submit error stays inline without closing the dialog.
 */
export const SubmitError: Story = {
  args: {
    ...baseArgs,
    submitError: "Provider codex rejected the selected reasoning effort.",
  },
};

/**
 * Explicit Live participation exposes only the named strategy accepted by Session creation.
 */
export const LiveParticipation: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    networkParticipation: liveParticipation,
  },
};

/** Pending admission keeps the dialog locked until the durable `201`. */
export const PendingAdmission: Story = {
  args: {
    ...baseArgs,
    isSubmitting: true,
  },
};
