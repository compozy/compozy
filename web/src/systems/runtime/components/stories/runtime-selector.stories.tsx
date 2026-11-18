import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { RuntimeSelector, runtimeModelKey } from "../runtime-selector";
import { FAVORITES_STORAGE_KEY } from "../runtime-selector/favorites";
import {
  runtimeSelectorStoryAdvancedOptions as advancedOptions,
  runtimeSelectorStoryClosedStates as closedStates,
  runtimeSelectorStoryLongNameModel as longNameModel,
  runtimeSelectorStoryManyIssueProviders as manyIssueProviders,
  runtimeSelectorStoryManyProviderIssues as manyProviderIssues,
  runtimeSelectorStoryModels as models,
  runtimeSelectorStoryOpenCodeProvider as openCodeProvider,
  runtimeSelectorStoryOverflowProviders as overflowProviders,
  runtimeSelectorStoryProviders as providers,
} from "./runtime-selector-story-fixtures";
import {
  ControlledRuntimeSelector,
  openPopup,
  RuntimeSelectorStoryFrame,
  storyDocs,
} from "./runtime-selector-story-scaffold";

const meta: Meta<typeof RuntimeSelector> = {
  title: "systems/runtime/components/RuntimeSelector",
  component: RuntimeSelector,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Unified provider · model · reasoning · speed selector (design ref: runtime-selector-variations.html, variation A). Closed it is a single button (provider mark, model name, reasoning meter, fast bolt); open it is a compact 320px dialog: search, a horizontal provider-chip radiogroup, a models listbox (combobox/activedescendant), and a one-line footer joining the reasoning slider with the ACP speed switch (PR #267 — rendered only when the surface wires it). Options arrive via props (data-agnostic). Fixtures are a truthful aggregate `view=all` set — including one canonical id shared across two providers.",
      },
    },
  },
  args: {
    providers,
    models,
    onChange: fn(),
    onRefreshCatalog: fn(),
    onOpenProviderSettings: fn(),
  },
  render: args => <RuntimeSelectorStoryFrame {...args} />,
};

export default meta;
type Story = StoryObj<typeof meta>;

// --- Closed-state trigger variants -----------------------------------------

/**
 * Every closed-state contract in one deterministic canvas: default, compact,
 * small, composer (idle and inert), no selectable reasoning, and provider
 * authentication needed.
 */
export const Default: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
  },
  parameters: storyDocs(
    "Closed-state contract: default, compact, small, composer, disabled composer, no-selectable-reasoning, and needs-auth variants."
  ),
  render: args => (
    <div className="mx-auto grid min-h-dvh w-full max-w-content-max content-start gap-3 p-6 sm:grid-cols-2">
      {closedStates.map(state => (
        <section
          key={state.label}
          className="flex min-h-32 flex-col justify-between gap-5 rounded-lg border border-line bg-canvas p-5"
        >
          <div>
            <h2 className="text-item-title font-semibold text-fg">{state.label}</h2>
            <p className="mt-1 text-small-body text-subtle">{state.description}</p>
          </div>
          <ControlledRuntimeSelector
            {...args}
            value={state.value}
            variant={state.variant}
            disabled={state.disabled}
          />
        </section>
      ))}
    </div>
  ),
};

// --- Open popup states ------------------------------------------------------

/** Open browse state: single-line rows plus the reasoning slider footer. */
export const PopupReasoningSlider: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
  },
  parameters: storyDocs(
    "Open popup. Rows are single-line (bare provider mark, name, brain indicator); the footer slider shows only the model's real levels — `none` is filtered out, there is no Default stop, and the model default (medium) renders pre-selected while the wire value stays empty."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    // Single-line row: the selected model renders with the neutral tint + check.
    const list = within(await body.findByTestId("runtime-selector-list"));
    const modelName = await list.findByText("GPT-5.6 Sol");
    const modelRow = modelName.closest<HTMLElement>('[role="option"]');
    if (!modelRow) throw new Error("model row was not rendered as a listbox option");
    await expect(modelRow).toHaveAttribute("data-selected", "true");
    // Slider footer settled: levels mode, none/Default absent, default pre-marked.
    const footer = await body.findByTestId("runtime-selector-reasoning");
    await expect(footer).toHaveAttribute("data-reasoning-mode", "levels");
    const track = await body.findByTestId("runtime-selector-reasoning-track");
    await expect(track).toHaveAttribute("aria-valuetext", "Medium (model default)");
    await expect(within(footer).queryByText("None")).not.toBeInTheDocument();
    await expect(within(footer).queryByText("Default")).not.toBeInTheDocument();
  },
};

/**
 * Truncated names and OpenCode provider labels keep their full text on hover;
 * the visual truncation stays, the tooltip is the reveal.
 */
export const PopupModelNameTooltip: Story = {
  args: {
    value: { provider: "opencode", model: "", reasoning_effort: "" },
    providers: [...providers, openCodeProvider],
    models: [...models, longNameModel],
  },
  parameters: storyDocs(
    "Hovering a truncated model name (or its OpenCode upstream provider label) reveals the complete text through the shared tooltip; neither trigger takes a tab stop inside the listbox."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    const list = within(await body.findByTestId("runtime-selector-list"));
    const name = await list.findByText("Claude Sonnet 4.6 Long Context Preview");
    await userEvent.hover(name);
    await expect(
      await body.findByText("Claude Sonnet 4.6 Long Context Preview", {
        selector: '[data-slot="tooltip-content"]',
      })
    ).toBeVisible();
  },
};

/**
 * More providers than the 320px rail can show: the native scrollbar is gone,
 * edge fades mark the hidden chips, and the chevron pages the track.
 */
export const PopupOverflowingRail: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    providers: overflowProviders,
  },
  parameters: storyDocs(
    "Overflowing provider rail: 28px chips inside the unchanged 32px line, no native scrollbar, and a pointer-only end chevron that pages the track without moving focus."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    const track = body.getByRole("radiogroup");
    await expect(track.scrollWidth).toBeGreaterThan(track.clientWidth);

    const end = await waitFor(() => {
      const edge = canvasElement.ownerDocument.body.querySelector<HTMLElement>(
        '[data-rail-scroll="end"]'
      );
      if (!edge) throw new Error("expected the rail end scroll affordance");
      return edge;
    });
    await expect(end).toBeVisible();

    const before = track.scrollLeft;
    await userEvent.click(end);
    await waitFor(() => expect(track.scrollLeft).toBeGreaterThan(before));
  },
};

/** Full-set search revealing a non-curated row while preserving exact tuple identity. */
export const PopupSearch: Story = {
  args: { value: { provider: "codex", model: "", reasoning_effort: "" } },
  parameters: storyDocs(
    "Search spans the full set (non-curated + cross-provider); the rail is suppressed."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await userEvent.type(body.getByTestId("runtime-selector-search"), "gpt");
    // Assert the filter actually settled before capture: the non-curated gpt row is
    // revealed and every non-matching Claude row is gone (no stale rows in the shot).
    const list = within(await body.findByTestId("runtime-selector-list"));
    await expect(await list.findByText("GPT-5.4 Mini")).toBeInTheDocument();
    await expect(list.queryByText("Claude Fable 5")).not.toBeInTheDocument();
    await expect(list.queryByText("Claude Haiku 4.5")).not.toBeInTheDocument();
  },
};

/** Model-supported reasoning where the provider exposes no selectable effort levels. */
export const PopupReasoningProviderDecides: Story = {
  args: { value: { provider: "claude", model: "claude-haiku-4-5-20251001", reasoning_effort: "" } },
  parameters: storyDocs(
    "Reasoning footer — supported without selectable levels ('provider decides')."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    // The 'provider decides' footer settled: supported-nolevels mode + the note text.
    const footer = await body.findByTestId("runtime-selector-reasoning");
    await expect(footer).toHaveAttribute("data-reasoning-mode", "supported-nolevels");
    await expect(within(footer).getByText(/provider decides/i)).toBeInTheDocument();
  },
};

/** Advertised non-dedicated ACP controls stay quiet until the disclosure opens. */
export const PopupAdvancedOptions: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    acpOptions: advancedOptions,
  },
  parameters: storyDocs(
    "Live ACP descriptors render as grouped selects and typed boolean controls under progressive disclosure; private transport aliases are not part of the fixture."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    const toggle = await body.findByTestId("runtime-selector-advanced-toggle");
    await expect(body.queryByTestId("runtime-selector-advanced-panel")).not.toBeInTheDocument();
    await userEvent.click(toggle);
    const panel = await body.findByTestId("runtime-selector-advanced-panel");
    await expect(within(panel).getByTestId("runtime-selector-option-context_window")).toHaveValue(
      "200k"
    );
    await expect(within(panel).getByTestId("runtime-selector-option-thinking")).toHaveAttribute(
      "aria-checked",
      "false"
    );
  },
};

/** ACP speed request (PR #267): the wired switch shares the footer line with the slider. */
export const PopupSpeedControl: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    onSpeedChange: fn(),
  },
  parameters: storyDocs(
    "Speed wired (session-create shape), resting on normal: the footer's Fast switch requests `speed: fast`; the daemon resolves it at dispatch, so the switch is intent — never a per-model capability claim."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    const speedSwitch = await body.findByTestId("runtime-selector-speed");
    await expect(speedSwitch).toHaveAttribute("aria-checked", "false");
  },
};

/** Fast engaged: the bolt thumb crosses the track and the closed trigger gains the bolt. */
export const PopupSpeedFast: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    onSpeedChange: fn(),
  },
  parameters: storyDocs(
    "Fast requested: the switch is checked (accent track, bolt on the thumb) and the closed trigger mirrors the request with the fast bolt beside the reasoning meter."
  ),
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    const speedSwitch = await body.findByTestId("runtime-selector-speed");
    await userEvent.click(speedSwitch);
    await expect(speedSwitch).toHaveAttribute("aria-checked", "true");
    // The closed-trigger bolt mirrors the active fast request.
    const trigger = within(canvasElement).getByRole("button", { name: /^Runtime:/ });
    await expect(trigger.querySelector('[data-slot="runtime-selector-fast"]')).not.toBeNull();
  },
};

/** Composer variant with the popup open — the trigger must stay chromeless. */
export const PopupComposerVariant: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
    variant: "composer",
  },
  parameters: storyDocs(
    "Open popup on the composer variant. The trigger keeps no border, fill, or shadow while open — only the chevron flips and the label brightens; the popup itself is identical to every other variant."
  ),
  play: async ({ canvasElement }) => {
    await openPopup(canvasElement);
    const trigger = within(canvasElement).getByRole("button", { name: /^Runtime:/ });
    await expect(trigger).toHaveAttribute("data-variant", "composer");
    await expect(trigger).toHaveAttribute("data-open", "true");
  },
};

/** Cross-provider favorites filtered from compound provider/model identities. */
export const PopupFavoritesRail: Story = {
  args: { value: { provider: "claude", model: "claude-fable-5", reasoning_effort: "high" } },
  parameters: storyDocs("Favorites rail filter — cross-provider favorites pinned."),
  // Deterministically seed cross-provider favorites (both are valid current tuples,
  // so they survive the strict reconcile); clean up so state never leaks to peers.
  beforeEach: () => {
    window.localStorage.setItem(
      FAVORITES_STORAGE_KEY,
      JSON.stringify([
        runtimeModelKey("codex", "gpt-5.6-sol"),
        runtimeModelKey("claude", "claude-fable-5"),
      ])
    );
    return () => window.localStorage.removeItem(FAVORITES_STORAGE_KEY);
  },
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await userEvent.click(body.getByRole("radio", { name: "Favorites" }));
    // Await the pinned cross-provider favorites inside the list before capture.
    const list = within(await body.findByTestId("runtime-selector-list"));
    await expect(await list.findByText("GPT-5.6 Sol")).toBeInTheDocument();
    await expect(list.getByText("Claude Fable 5")).toBeInTheDocument();
  },
};

/** One provider needs sign-in: the header carries the warning mark before refresh. */
export const PopupCatalogIssue: Story = {
  tags: ["play-fn"],
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    providerCatalogIssues: { cursor: "Sign in to update the model list." },
  },
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await expect(body.getByRole("button", { name: "Model list issues" })).toBeInTheDocument();
    await expect(body.queryByText("Sign in to update the model list.")).not.toBeInTheDocument();
  },
};

/** The warning mark's popover lists each provider's model list problem in provider order. */
export const PopupCatalogIssuesOpen: Story = {
  tags: ["play-fn"],
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    providerCatalogIssues: {
      claude: "The model list took too long to update. Try refreshing again.",
      openrouter: "Could not update the model list. Try refreshing again.",
      cursor: "Sign in to update the model list.",
    },
  },
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await userEvent.click(body.getByRole("button", { name: "Model list issues" }));
    const issues = within(await body.findByTestId("runtime-selector-catalog-issues-popup"));
    await expect(issues.getAllByRole("listitem")).toHaveLength(3);
    await expect(issues.getByText("Sign in to update the model list.")).toBeInTheDocument();
  },
};

/** Many failing providers: the issues popover scrolls inside its own bound. */
export const PopupManyCatalogIssuesOpen: Story = {
  tags: ["play-fn"],
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    providers: manyIssueProviders,
    providerCatalogIssues: manyProviderIssues,
  },
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await userEvent.click(body.getByRole("button", { name: "Model list issues" }));
    const issues = await body.findByTestId("runtime-selector-catalog-issues-popup");
    await expect(within(issues).getAllByRole("listitem")).toHaveLength(13);
    await expect(issues.scrollHeight).toBeGreaterThan(issues.clientHeight);
  },
};

export const ProviderAuthRequired: Story = {
  tags: ["play-fn"],
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
    providers: manyIssueProviders,
    providerCatalogIssues: manyProviderIssues,
  },
  // Invariant: discovery guidance stays behind the header mark (issue #827), its nested
  // popover dismisses before the selector, and no guidance covers a healthy model's
  // pointer target. Owner: rendered RuntimeSelector; canonical suite: this browser story.
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    await expect(body.queryByText("Sign in to update the model list.")).not.toBeInTheDocument();
    await userEvent.click(body.getByRole("button", { name: "Model list issues" }));
    const issues = await body.findByTestId("runtime-selector-catalog-issues-popup");
    await expect(within(issues).getAllByRole("listitem")).toHaveLength(13);
    await userEvent.keyboard("{Escape}");
    await waitFor(() =>
      expect(body.queryByTestId("runtime-selector-catalog-issues-popup")).not.toBeInTheDocument()
    );
    await expect(body.getByTestId("runtime-selector-popup")).toBeInTheDocument();

    await userEvent.type(body.getByTestId("runtime-selector-search"), "GPT-5.6 Sol");
    const option = body.getByRole("option", { name: /GPT-5.6 Sol from Codex/ });
    option.scrollIntoView({ block: "nearest" });
    await waitFor(async () => {
      const bounds = option.getBoundingClientRect();
      const hit = option.ownerDocument.elementFromPoint(
        bounds.x + bounds.width / 2,
        bounds.y + bounds.height / 2
      );
      await expect(option.contains(hit)).toBe(true);
    });
    await userEvent.click(option);
    await expect(body.getByTestId("runtime-selector-reasoning")).toHaveAttribute(
      "data-reasoning-mode",
      "levels"
    );
  },
};
