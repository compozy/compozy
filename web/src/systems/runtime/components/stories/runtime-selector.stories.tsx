import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import {
  RuntimeSelector,
  runtimeModelKey,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorProps,
  type RuntimeSelectorValue,
} from "../runtime-selector";
import { FAVORITES_STORAGE_KEY } from "../runtime-selector/favorites";

// Truthful aggregate fixtures: two live providers plus a signed-out one, and a
// cross-provider model set carrying real curation / availability / reasoning
// metadata (a shared canonical id `gpt-5.6-sol` proves compound identity).
const providers: RuntimeProviderOption[] = [
  { id: "codex", name: "Codex", runtime_provider: "codex", harness: "acp" },
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
  { id: "openrouter", name: "OpenRouter", runtime_provider: "openrouter", harness: "pi_acp" },
  { id: "cursor", name: "Cursor", runtime_provider: "cursor", harness: "acp", needs_auth: true },
];

function m(overrides: Partial<RuntimeModelOption> & { id: string; provider: string }) {
  return {
    name: overrides.id,
    efforts: [],
    availability: "live",
    curated: true,
    supports_tools: true,
    ...overrides,
  } satisfies RuntimeModelOption;
}

const models: RuntimeModelOption[] = [
  m({
    id: "gpt-5.6-sol",
    provider: "codex",
    name: "GPT-5.6 Sol",
    featured: true,
    context_window: 1_050_000,
    cost_input: 5,
    cost_output: 30,
    efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_effort: "medium",
    reasoning_source: "acp",
    release_date: "2026-06-26",
  }),
  m({
    id: "gpt-5.6-luna",
    provider: "codex",
    name: "GPT-5.6 Luna",
    context_window: 1_050_000,
    cost_input: 1,
    cost_output: 6,
    efforts: ["none", "low", "medium", "high", "xhigh", "max"],
    default_effort: "medium",
    reasoning_source: "acp",
  }),
  m({
    id: "claude-fable-5",
    provider: "claude",
    name: "Claude Fable 5",
    featured: true,
    context_window: 1_000_000,
    cost_input: 10,
    cost_output: 50,
    efforts: ["low", "medium", "high", "xhigh", "max"],
    default_effort: "high",
    reasoning_source: "acp",
    release_date: "2026-06-09",
  }),
  m({
    id: "claude-haiku-4-5-20251001",
    provider: "claude",
    name: "Claude Haiku 4.5",
    context_window: 200_000,
    cost_input: 1,
    cost_output: 5,
    // Canonical builtin metadata: supports_reasoning with no selectable effort
    // subset → the trigger drops its reasoning segment and the footer reads
    // "provider decides" (internal/config/provider_reasoning.go).
    supports_reasoning: true,
  }),
  m({
    id: "gpt-5.4-mini",
    provider: "codex",
    name: "GPT-5.4 Mini",
    // Non-curated: hidden while browsing, revealed on search.
    curated: false,
    availability: "stale",
  }),
  // Same canonical id under a different provider — a distinct, independently
  // selectable/favoritable row (compound identity).
  m({
    id: "gpt-5.6-sol",
    provider: "openrouter",
    name: "GPT-5.6 Sol (via OpenRouter)",
    context_window: 1_050_000,
    cost_input: 6,
    cost_output: 32,
    efforts: ["low", "medium", "high"],
    reasoning_source: "catalog",
  }),
];

function ControlledRuntimeSelector({ value: initial, ...props }: RuntimeSelectorProps) {
  const [value, setValue] = useState<RuntimeSelectorValue>(initial);
  return (
    <RuntimeSelector
      {...props}
      value={value}
      onChange={next => {
        props.onChange(next);
        setValue(next);
      }}
    />
  );
}

function RuntimeSelectorStoryFrame(props: RuntimeSelectorProps) {
  return (
    // Fullscreen story envelope (no arbitrary fixed geometry); the popup portals to
    // the body and floats from the trigger, so the frame just needs viewport height.
    <div className="flex min-h-dvh items-start p-6">
      <ControlledRuntimeSelector {...props} />
    </div>
  );
}

const meta: Meta<typeof RuntimeSelector> = {
  title: "systems/runtime/components/RuntimeSelector",
  component: RuntimeSelector,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Unified provider · model · reasoning selector. Closed it is a single button (provider mark, model name, reasoning meter); open it is a named dialog with a provider-filter radiogroup, a models listbox (combobox/activedescendant), and a reasoning slider footer. Options arrive via props (data-agnostic). Fixtures are a truthful aggregate `view=all` set — including one canonical id shared across two providers.",
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

async function openPopup(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  await userEvent.click(canvas.getByRole("button", { name: /^Runtime:/ }));
  const body = within(canvasElement.ownerDocument.body);
  await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  return body;
}

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
  parameters: {
    docs: {
      description: {
        story:
          "Closed-state contract: default, compact, small, composer, disabled composer, no-selectable-reasoning, and needs-auth variants.",
      },
    },
  },
  render: args => {
    const states: Array<{
      label: string;
      description: string;
      value: RuntimeSelectorValue;
      variant?: RuntimeSelectorProps["variant"];
      disabled?: boolean;
    }> = [
      {
        label: "Default",
        description: "Provider, model, and explicit effort",
        value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
      },
      {
        label: "Compact",
        description: "Provider glyph for dense toolbars",
        value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
        variant: "compact",
      },
      {
        label: "Small",
        description: "Provider default in a tighter form row",
        value: { provider: "claude", model: "claude-fable-5", reasoning_effort: "" },
        variant: "small",
      },
      {
        label: "Composer",
        description: "Chromeless inside a prompt box: no border, fill, or shadow",
        value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
        variant: "composer",
      },
      {
        label: "Composer, inert",
        description: "Dimmed and unpressable: label and chevron stop answering hover",
        value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
        variant: "composer",
        disabled: true,
      },
      {
        label: "No selectable effort",
        description: "Reasoning meter is absent",
        value: {
          provider: "claude",
          model: "claude-haiku-4-5-20251001",
          reasoning_effort: "",
        },
      },
      {
        label: "Needs sign in",
        description: "Unavailable provider is stated inline",
        value: { provider: "cursor", model: "", reasoning_effort: "" },
      },
    ];

    return (
      <div className="mx-auto grid min-h-dvh w-full max-w-content-max content-start gap-3 p-6 sm:grid-cols-2">
        {states.map(state => (
          <section
            key={state.label}
            className="flex min-h-32 flex-col justify-between gap-5 rounded-lg border border-line bg-canvas-soft p-5"
          >
            <div>
              <h2 className="text-item-title font-semibold text-fg-strong">{state.label}</h2>
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
    );
  },
};

// --- Open popup states ------------------------------------------------------

/** Open browse state: single-line rows plus the reasoning slider footer. */
export const PopupReasoningSlider: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "" },
  },
  parameters: {
    docs: {
      description: {
        story:
          "Open popup. Rows are single-line (bare provider mark, name, brain indicator); the footer slider shows only the model's real levels — `none` is filtered out, there is no Default stop, and the model default (medium) renders pre-selected while the wire value stays empty.",
      },
    },
  },
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

/** Full-set search revealing a non-curated row while preserving exact tuple identity. */
export const PopupSearch: Story = {
  args: { value: { provider: "codex", model: "", reasoning_effort: "" } },
  parameters: {
    docs: {
      description: {
        story: "Search spans the full set (non-curated + cross-provider); the rail is suppressed.",
      },
    },
  },
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
  parameters: {
    docs: {
      description: {
        story: "Reasoning footer — supported without selectable levels ('provider decides').",
      },
    },
  },
  play: async ({ canvasElement }) => {
    const body = await openPopup(canvasElement);
    // The 'provider decides' footer settled: supported-nolevels mode + the note text.
    const footer = await body.findByTestId("runtime-selector-reasoning");
    await expect(footer).toHaveAttribute("data-reasoning-mode", "supported-nolevels");
    await expect(within(footer).getByText(/expose selectable effort/i)).toBeInTheDocument();
  },
};

/** Composer variant with the popup open — the trigger must stay chromeless. */
export const PopupComposerVariant: Story = {
  args: {
    value: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
    variant: "composer",
  },
  parameters: {
    docs: {
      description: {
        story:
          "Open popup on the composer variant. The trigger keeps no border, fill, or shadow while open — only the chevron flips and the label brightens; the popup itself is identical to every other variant.",
      },
    },
  },
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
  parameters: {
    docs: { description: { story: "Favorites rail filter — cross-provider favorites pinned." } },
  },
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
