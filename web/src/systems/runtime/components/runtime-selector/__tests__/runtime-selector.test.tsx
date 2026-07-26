import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { IntensityMeter, UIProvider } from "@agh/ui";

import { FAVORITES_STORAGE_KEY, RECENTS_LIMIT, RECENTS_STORAGE_KEY } from "../favorites";
import { runtimeModelKey } from "../model-key";
import { ReasoningBar } from "../reasoning-bar";
import { RuntimeSelector, type RuntimeSelectorProps } from "../runtime-selector";
import { useRuntimeFavorites } from "../use-runtime-favorites";
import {
  reasoningEffortPosition,
  REASONING_EFFORT_ORDER,
  resolveReasoningState,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "../types";

// ---------------------------------------------------------------------------
// Fixtures + harness
// ---------------------------------------------------------------------------

const codexProvider: RuntimeProviderOption = {
  id: "codex",
  name: "Codex",
  runtime_provider: "codex",
};
const claudeProvider: RuntimeProviderOption = {
  id: "claude",
  name: "Claude",
  runtime_provider: "claude",
};

// Browser-local favorites/recents must not leak across cases (each seeds its own).
beforeEach(() => window.localStorage.clear());

function model(id: string, overrides: Partial<RuntimeModelOption> = {}): RuntimeModelOption {
  return {
    id,
    provider: "codex",
    name: id,
    efforts: [],
    availability: "live",
    curated: true,
    ...overrides,
  };
}

interface HarnessOptions {
  value: RuntimeSelectorValue;
  providers?: RuntimeProviderOption[];
  models?: RuntimeModelOption[];
  props?: Partial<RuntimeSelectorProps>;
}

function renderSelector({
  value,
  providers = [codexProvider],
  models = [],
  props,
}: HarnessOptions) {
  const onChange = vi.fn<(next: RuntimeSelectorValue) => void>();
  function Harness() {
    const [current, setCurrent] = useState<RuntimeSelectorValue>(value);
    return (
      <UIProvider reducedMotion="always">
        <RuntimeSelector
          {...props}
          value={current}
          onChange={next => {
            onChange(next);
            setCurrent(next);
          }}
          providers={providers}
          models={models}
          triggerTestId="rt-trigger"
        />
      </UIProvider>
    );
  }
  const result = render(<Harness />);
  return { onChange, unmount: result.unmount };
}

async function openSelector(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByTestId("rt-trigger"));
  return screen.findByTestId("runtime-selector-popup");
}

function row(id: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(`[data-model="${id}"]`);
  if (!el) throw new Error(`Missing model row "${id}"`);
  return el;
}

function optionOrder(): string[] {
  return screen
    .getAllByRole("option")
    .map(node => node.getAttribute("data-model") ?? "")
    .filter(Boolean);
}

describe("RuntimeSelector read-only contract", () => {
  it("Should keep the trigger focusable and aria-disabled while preventing the popup from opening", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "gpt-a", reasoning_effort: "" },
      models: [model("gpt-a")],
      props: { readOnly: true },
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger).toHaveAttribute("aria-disabled", "true");
    expect(trigger).not.toBeDisabled();
    trigger.focus();
    expect(trigger).toHaveFocus();
    await user.click(trigger);
    expect(screen.queryByTestId("runtime-selector-popup")).toBeNull();
    expect(onChange).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Single-button trigger
// ---------------------------------------------------------------------------

describe("RuntimeSelector single-button trigger", () => {
  it("Should render one click surface that toggles the popup open and closed", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "gpt-a", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger.tagName).toBe("BUTTON");
    // The whole closed selector is ONE button — no nested segment buttons.
    expect(trigger.querySelector("button")).toBeNull();

    await user.click(trigger);
    expect(await screen.findByTestId("runtime-selector-popup")).toBeInTheDocument();
    expect(trigger).toHaveAttribute("data-open", "true");

    await user.click(trigger);
    await waitFor(() =>
      expect(screen.queryByTestId("runtime-selector-popup")).not.toBeInTheDocument()
    );
    expect(trigger).toHaveAttribute("data-open", "false");
  });

  it.each([
    ["Meta", "metaKey"],
    ["Control", "ctrlKey"],
  ] as const)(
    "Should consume %s+J on the focused trigger before an ancestor shortcut boundary",
    async (_modifierName, modifier) => {
      const ancestorShortcutBoundary = vi.fn();
      function Instance() {
        const [value, setValue] = useState<RuntimeSelectorValue>({
          provider: "codex",
          model: "",
          reasoning_effort: "",
        });
        return (
          <RuntimeSelector
            value={value}
            onChange={setValue}
            providers={[codexProvider]}
            models={[]}
            triggerTestId="rt-focused"
          />
        );
      }
      render(
        <UIProvider reducedMotion="always">
          <div
            onKeyDown={event => {
              ancestorShortcutBoundary(event.key);
              event.stopPropagation();
            }}
          >
            <Instance />
          </div>
        </UIProvider>
      );

      const trigger = screen.getByTestId("rt-focused");
      trigger.focus();

      const unrelatedAccepted = fireEvent.keyDown(trigger, {
        key: "k",
        [modifier]: true,
      });
      expect(unrelatedAccepted).toBe(true);
      expect(ancestorShortcutBoundary).toHaveBeenCalledOnce();
      expect(screen.queryByTestId("runtime-selector-popup")).not.toBeInTheDocument();

      ancestorShortcutBoundary.mockClear();
      const runtimeAccepted = fireEvent.keyDown(trigger, {
        key: "j",
        [modifier]: true,
      });

      expect(runtimeAccepted).toBe(false);
      expect(ancestorShortcutBoundary).not.toHaveBeenCalled();
      await waitFor(() => expect(screen.getByTestId("runtime-selector-popup")).toBeVisible());
    }
  );

  it("Should show only the model name as visible text — provider name and effort label live in the popup", () => {
    renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "high" },
      models: [model("leveled", { name: "Leveled", efforts: ["low", "medium", "high"] })],
    });

    const trigger = screen.getByTestId("rt-trigger");
    // The model name is the only surface text: no provider name, no effort label
    // (the provider mark's embedded brand text is icon-internal, not a label).
    expect(within(trigger).getByText("Leveled")).toBeInTheDocument();
    expect(within(trigger).queryByText("Codex")).toBeNull();
    expect(within(trigger).queryByText("High")).toBeNull();
    // The full identity still reaches assistive tech through the accessible name.
    expect(trigger).toHaveAccessibleName("Runtime: Codex / Leveled, reasoning High");
  });

  it("Should not advertise any keyboard shortcut on the trigger", () => {
    renderSelector({
      value: { provider: "codex", model: "gpt-a", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger).not.toHaveAttribute("aria-keyshortcuts");
    expect(trigger.querySelector("kbd")).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// Reset-on-switch
// ---------------------------------------------------------------------------

describe("RuntimeSelector reasoning reset-on-switch", () => {
  it("Should reset reasoning_effort to '' when the picked model cannot honor the current effort", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "with-high", reasoning_effort: "high" },
      models: [
        model("with-high", { name: "With High", efforts: ["low", "medium", "high"] }),
        model("no-high", { name: "No High", efforts: ["low", "medium"] }),
      ],
    });

    await openSelector(user);
    await user.click(row("no-high"));

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "no-high",
      reasoning_effort: "",
    });
  });

  it("Should keep the current reasoning_effort when the picked model still supports it", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "with-high", reasoning_effort: "high" },
      models: [
        model("with-high", { name: "With High", efforts: ["low", "medium", "high"] }),
        model("also-high", { name: "Also High", efforts: ["medium", "high", "xhigh"] }),
      ],
    });

    await openSelector(user);
    await user.click(row("also-high"));

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "also-high",
      reasoning_effort: "high",
    });
  });
});

// ---------------------------------------------------------------------------
// Reasoning trigger segment + footer
// ---------------------------------------------------------------------------

describe("RuntimeSelector reasoning trigger + footer", () => {
  it("Should hide the trigger meter when the selected model exposes no efforts", () => {
    renderSelector({
      value: { provider: "codex", model: "plain", reasoning_effort: "" },
      models: [model("plain", { name: "Plain", efforts: [] })],
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger.querySelector('[data-slot="intensity-meter"]')).toBeNull();
  });

  it("Should fill the trigger meter with the model default while reasoning is unset", () => {
    renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [
        model("leveled", {
          name: "Leveled",
          efforts: ["low", "medium", "high"],
          default_effort: "medium",
        }),
      ],
    });

    const trigger = screen.getByTestId("rt-trigger");
    const meter = trigger.querySelector('[data-slot="intensity-meter"]');
    expect(meter).not.toBeNull();
    // The meter mirrors the slider: the model default (medium → canonical
    // position 4) fills the bars while the wire value stays "". No effort
    // label text renders on the trigger.
    expect(meter).toHaveAttribute("data-hollow", "false");
    expect(meter).toHaveAttribute("data-position", "4");
    // No effort label text renders on the trigger — the meter is the only cue.
    expect(within(trigger).queryByText("Medium")).toBeNull();
  });

  it("Should render a hollow trigger meter when reasoning is unset and the model has no default", () => {
    renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [model("leveled", { name: "Leveled", efforts: ["low", "medium", "high"] })],
    });

    const meter = screen.getByTestId("rt-trigger").querySelector('[data-slot="intensity-meter"]');
    expect(meter).not.toBeNull();
    // Semantic state (hollow, zero bars filled), not the visual class.
    expect(meter).toHaveAttribute("data-hollow", "true");
    expect(meter).toHaveAttribute("data-position", "0");
  });

  it("Should render only the model's efforts as slider stops in canonical order — no None, no Default", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [
        model("leveled", {
          name: "Leveled",
          efforts: ["none", "high", "low", "xhigh", "medium"],
        }),
      ],
    });

    await openSelector(user);

    const strip = screen.getByTestId("runtime-selector-reasoning");
    const buttons = Array.from(strip.querySelectorAll<HTMLElement>("button[data-rz]"));
    // `none` is filtered out and there is no "" (Default) stop — the first
    // stop is the lowest real level.
    expect(buttons.map(button => button.getAttribute("data-rz"))).toEqual([
      "low",
      "medium",
      "high",
      "xhigh",
    ]);
    expect(within(strip).queryByText("Default")).not.toBeInTheDocument();
    expect(within(strip).queryByText("None")).not.toBeInTheDocument();
  });

  it("Should tag the reasoning footer source badge as ACP for acp-sourced reasoning", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "acp-model", reasoning_effort: "" },
      models: [
        model("acp-model", {
          name: "ACP Model",
          efforts: ["low", "high"],
          reasoning_source: "acp",
        }),
      ],
    });

    await openSelector(user);
    expect(
      within(screen.getByTestId("runtime-selector-reasoning")).getByText("ACP")
    ).toBeInTheDocument();
  });

  it("Should tag the reasoning footer source badge as CATALOG for catalog-sourced reasoning", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "cat-model", reasoning_effort: "" },
      models: [
        model("cat-model", {
          name: "Catalog Model",
          efforts: ["low", "high"],
          reasoning_source: "catalog",
        }),
      ],
    });

    await openSelector(user);
    expect(
      within(screen.getByTestId("runtime-selector-reasoning")).getByText("CATALOG")
    ).toBeInTheDocument();
  });

  it("Should emit the chosen effort when a reasoning footer level is clicked", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [model("leveled", { name: "Leveled", efforts: ["low", "medium", "high"] })],
    });

    await openSelector(user);
    await user.click(
      screen.getByTestId("runtime-selector-reasoning").querySelector('button[data-rz="high"]')!
    );

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "high",
    });
  });
});

// ---------------------------------------------------------------------------
// Reasoning slider
// ---------------------------------------------------------------------------

describe("RuntimeSelector reasoning slider", () => {
  const sliderTrack = () => screen.getByTestId("runtime-selector-reasoning-track");

  it("Should preselect the model default while the wire value stays ''", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [
        model("leveled", {
          name: "Leveled",
          efforts: ["low", "medium", "high"],
          default_effort: "medium",
        }),
      ],
    });

    await openSelector(user);

    // The default reads as selected (accent, marked label) without ever
    // emitting a change — "" still means provider default on the wire.
    const track = sliderTrack();
    expect(track).toHaveAttribute("aria-valuenow", "1");
    expect(track).toHaveAttribute("aria-valuetext", "Medium (model default)");
    expect(document.querySelector('button[data-rz="medium"]')).toHaveAttribute("data-on", "true");
    expect(screen.getByTestId("runtime-selector-reasoning-slider")).toHaveAttribute(
      "data-unset",
      "false"
    );
    expect(onChange).not.toHaveBeenCalled();
  });

  it("Should reflect an explicit wire value on the slider", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "high" },
      models: [model("leveled", { name: "Leveled", efforts: ["low", "medium", "high"] })],
    });

    await openSelector(user);

    const track = sliderTrack();
    expect(track).toHaveAttribute("aria-valuenow", "2");
    expect(track).toHaveAttribute("aria-valuetext", "High");
  });

  it("Should step levels with arrow keys and jump with Home/End on the slider track", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [
        model("leveled", {
          name: "Leveled",
          efforts: ["low", "medium", "high"],
          default_effort: "medium",
        }),
      ],
    });

    await openSelector(user);

    // Stepping starts from the displayed default (medium).
    fireEvent.keyDown(sliderTrack(), { key: "ArrowRight" });
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "high",
    });

    fireEvent.keyDown(sliderTrack(), { key: "ArrowLeft" });
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "medium",
    });

    fireEvent.keyDown(sliderTrack(), { key: "Home" });
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "low",
    });

    fireEvent.keyDown(sliderTrack(), { key: "End" });
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "high",
    });
  });

  it("Should commit the default level explicitly when its stop label is clicked", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [
        model("leveled", {
          name: "Leveled",
          efforts: ["low", "medium", "high"],
          default_effort: "medium",
        }),
      ],
    });

    await openSelector(user);
    await user.click(document.querySelector<HTMLElement>('button[data-rz="medium"]')!);

    // Clicking the already-displayed default is an explicit pick: the level
    // goes on the wire instead of remaining "".
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "medium",
    });
  });

  it("Should rest unset when the model has no default and select the first level on an arrow key", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "leveled", reasoning_effort: "" },
      models: [model("leveled", { name: "Leveled", efforts: ["low", "medium", "high"] })],
    });

    await openSelector(user);

    expect(screen.getByTestId("runtime-selector-reasoning-slider")).toHaveAttribute(
      "data-unset",
      "true"
    );
    expect(sliderTrack()).toHaveAttribute("aria-valuetext", "Provider default");

    fireEvent.keyDown(sliderTrack(), { key: "ArrowRight" });
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "leveled",
      reasoning_effort: "low",
    });
  });
});

// ---------------------------------------------------------------------------
// Needs-auth provider
// ---------------------------------------------------------------------------

describe("RuntimeSelector needs-auth provider", () => {
  const authProvider: RuntimeProviderOption = {
    id: "codex",
    name: "Codex",
    runtime_provider: "codex",
    needs_auth: true,
  };
  const authModels = [
    model("gpt-a", {
      name: "GPT A",
      availability: "unavailable",
      disabled: true,
      disabled_reason: "Sign in",
    }),
    model("gpt-b", {
      name: "GPT B",
      availability: "unavailable",
      disabled: true,
      disabled_reason: "Sign in",
    }),
  ];

  it("Should surface the sign-in warning on the trigger for a needs-auth provider", () => {
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [authProvider],
      models: authModels,
    });

    expect(screen.getByRole("img", { name: "Provider needs sign in" })).toBeInTheDocument();
  });

  it("Should expose a model-unavailable warning to assistive tech on the trigger", () => {
    renderSelector({
      value: { provider: "codex", model: "gone", reasoning_effort: "" },
      models: [
        model("gone", {
          name: "Gone",
          availability: "unavailable",
          disabled: true,
          disabled_reason: "Unavailable",
        }),
      ],
    });

    expect(screen.getByRole("img", { name: "Model unavailable" })).toBeInTheDocument();
  });

  it("Should dim the rail item and disable every row with a reason for a needs-auth provider", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [authProvider],
      models: authModels,
    });

    await openSelector(user);

    expect(document.querySelector('[data-rail="codex"]')).toHaveAttribute("data-dim", "true");
    expect(row("gpt-a")).toHaveAttribute("data-disabled", "true");
    expect(row("gpt-a")).toHaveTextContent("Sign in");
  });

  it("Should not emit onChange when a disabled row is clicked", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [authProvider],
      models: authModels,
    });

    await openSelector(user);
    await user.click(row("gpt-a"));

    expect(onChange).not.toHaveBeenCalled();
  });
});

describe("RuntimeSelector group availability", () => {
  it("Should label a group Unavailable when every model is unavailable", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [
        model("gone-a", {
          name: "Gone A",
          availability: "unavailable",
          disabled: true,
          disabled_reason: "Offline",
        }),
        model("gone-b", {
          name: "Gone B",
          availability: "unavailable",
          disabled: true,
          disabled_reason: "Offline",
        }),
      ],
    });

    await openSelector(user);

    const groupHead = screen
      .getByTestId("runtime-selector-list")
      .querySelector('[role="presentation"]');
    expect(groupHead).toHaveTextContent("Unavailable");
    expect(groupHead).not.toHaveTextContent("Live");
  });

  it("Should keep Stale ahead of Unavailable when any model is stale", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [
        model("stale-a", { name: "Stale A", availability: "stale" }),
        model("gone-b", {
          name: "Gone B",
          availability: "unavailable",
          disabled: true,
          disabled_reason: "Offline",
        }),
      ],
    });

    await openSelector(user);

    const groupHead = screen
      .getByTestId("runtime-selector-list")
      .querySelector('[role="presentation"]');
    expect(groupHead).toHaveTextContent("Stale");
    expect(groupHead).not.toHaveTextContent("Unavailable");
  });
});

// ---------------------------------------------------------------------------
// Custom model ID
// ---------------------------------------------------------------------------

describe("RuntimeSelector custom model id", () => {
  it("Should commit an unknown query as a custom model id when the custom button is clicked", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("known", { name: "Known" })],
    });

    await openSelector(user);
    fireEvent.change(screen.getByTestId("runtime-selector-search"), {
      target: { value: "custom-unknown-model" },
    });
    await user.click(await screen.findByTestId("runtime-selector-custom"));

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "custom-unknown-model",
      reasoning_effort: "",
    });
  });

  it("Should commit an unknown query as a custom model id on Enter", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("known", { name: "Known" })],
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    fireEvent.change(search, { target: { value: "typed-custom-id" } });
    fireEvent.keyDown(search, { key: "Enter" });

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "typed-custom-id",
      reasoning_effort: "",
    });
  });

  it("Should not emit a custom id when the query is empty", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("known", { name: "Known" })],
    });

    await openSelector(user);
    const custom = await screen.findByTestId("runtime-selector-custom");
    expect(custom).toHaveTextContent("Use an exact custom model ID…");
    await user.click(custom);

    expect(onChange).not.toHaveBeenCalled();
  });

  it("Should not offer a custom commit when no explicit provider is active", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      // All rail active with no selected provider → there is no explicit target.
      value: { provider: "", model: "", reasoning_effort: "" },
      providers: [codexProvider, claudeProvider],
      models: [model("known", { name: "Known" })],
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    fireEvent.change(search, { target: { value: "custom-unknown-model" } });

    // No provider target → the affordance is hidden and Enter emits nothing.
    await waitFor(() =>
      expect(screen.queryByTestId("runtime-selector-custom")).not.toBeInTheDocument()
    );
    fireEvent.keyDown(search, { key: "Enter" });
    expect(onChange).not.toHaveBeenCalled();
  });

  it("Should offer a custom commit for the active provider even when the id is known under another provider", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [codexProvider, claudeProvider],
      // "shared" exists ONLY under Claude — it must not block "shared" as a custom Codex id.
      models: [model("shared", { provider: "claude", name: "Shared on Claude" })],
    });

    await openSelector(user);
    fireEvent.change(screen.getByTestId("runtime-selector-search"), {
      target: { value: "shared" },
    });
    const custom = await screen.findByTestId("runtime-selector-custom");
    expect(custom).toHaveTextContent('Use "shared"');
    await user.click(custom);

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "shared",
      reasoning_effort: "",
    });
  });

  it("Should not offer a custom commit when the id exactly matches a model under the active provider", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [codexProvider],
      models: [model("gpt-5.6", { name: "GPT 5.6" })],
    });

    await openSelector(user);
    fireEvent.change(screen.getByTestId("runtime-selector-search"), {
      target: { value: "gpt-5.6" },
    });

    // Exact (codex, gpt-5.6) match → the real row is offered, never a custom commit.
    await waitFor(() => expect(row("gpt-5.6")).toBeInTheDocument());
    const custom = screen.getByTestId("runtime-selector-custom");
    expect(custom).toHaveTextContent("Use an exact custom model ID…");
    expect(custom).not.toHaveTextContent('Use "gpt-5.6"');
  });

  it("Should emit the exact rail-selected provider for a custom id", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "", model: "", reasoning_effort: "" },
      providers: [codexProvider, claudeProvider],
      models: [model("codex-a", { provider: "codex", name: "Codex A" })],
    });

    await openSelector(user);
    // Pick the Claude provider rail — the custom target is now explicitly Claude.
    await user.click(document.querySelector('[data-rail="claude"]')!);
    fireEvent.change(screen.getByTestId("runtime-selector-search"), {
      target: { value: "custom-z" },
    });
    await user.click(await screen.findByTestId("runtime-selector-custom"));

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "claude",
      model: "custom-z",
      reasoning_effort: "",
    });
  });
});

// ---------------------------------------------------------------------------
// Browse vs search + ranking
// ---------------------------------------------------------------------------

describe("RuntimeSelector browse, search and ranking", () => {
  const rankingModels = [
    model("gpt-plain", { name: "GPT Plain" }),
    model("gpt-featured", { name: "GPT Featured", featured: true }),
    model("gpt-fav", { name: "GPT Fav" }),
    model("gpt-hidden", { name: "GPT Hidden", curated: false }),
  ];

  it("Should show only curated rows while browsing and reveal non-curated rows on search", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: rankingModels,
    });

    await openSelector(user);

    // Browse: curated visible, non-curated hidden.
    expect(document.querySelector('[data-model="gpt-plain"]')).toBeInTheDocument();
    expect(document.querySelector('[data-model="gpt-hidden"]')).not.toBeInTheDocument();

    // Search: non-curated match now appears.
    fireEvent.change(screen.getByTestId("runtime-selector-search"), { target: { value: "gpt" } });
    await waitFor(() =>
      expect(document.querySelector('[data-model="gpt-hidden"]')).toBeInTheDocument()
    );
  });

  it("Should order rows favorites first, then featured, then daemon order", async () => {
    window.localStorage.setItem(
      FAVORITES_STORAGE_KEY,
      JSON.stringify([runtimeModelKey("codex", "gpt-fav")])
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: rankingModels,
    });

    await openSelector(user);
    // Drop the pinned "Recent & favorites" block so the provider group is the only listbox content.
    await user.click(document.querySelector('[data-rail="codex"]')!);

    await waitFor(() => expect(optionOrder()).toEqual(["gpt-fav", "gpt-featured", "gpt-plain"]));
  });
});

// ---------------------------------------------------------------------------
// Favorites + recents persistence
// ---------------------------------------------------------------------------

describe("RuntimeSelector favorites and recents persistence", () => {
  function readList(key: string): string[] {
    const raw = window.localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as string[]) : [];
  }

  it("Should persist a favorite toggle via the external favorite action (pointer)", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    // Hover activates the row; the real external favorite button acts on it — no
    // interactive control is nested inside the listbox option.
    await user.hover(row("gpt-a"));
    await user.click(screen.getByTestId("runtime-selector-favorite-action"));

    expect(readList(FAVORITES_STORAGE_KEY)).toContain(runtimeModelKey("codex", "gpt-a"));
    expect(row("gpt-a")).toHaveAttribute("data-favorite", "true");
  });

  it("Should expose the favorite action as a real toggle button with no interactive control nested in options", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    // ARIA-valid list: a listbox option wraps no button / role=button descendant.
    expect(row("gpt-a").querySelector("button, [role='button']")).toBeNull();

    await user.hover(row("gpt-a"));
    const favButton = screen.getByTestId("runtime-selector-favorite-action");
    expect(favButton.tagName).toBe("BUTTON");
    // A real toggle button whose accessible name is STABLE (includes provider
    // identity) and does NOT flip on toggle — favorite truth rides on aria-pressed.
    expect(favButton).toHaveAttribute("aria-pressed", "false");
    expect(favButton).toHaveAccessibleName("Favorite GPT A from Codex");

    await user.click(favButton);
    // Toggling flips aria-pressed; the accessible name stays identical.
    expect(favButton).toHaveAttribute("aria-pressed", "true");
    expect(favButton).toHaveAccessibleName("Favorite GPT A from Codex");
  });

  it("Should keep the favorite action disabled for a disabled row", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [
        model("gpt-disabled", {
          name: "GPT Disabled",
          disabled: true,
          disabled_reason: "Unavailable",
        }),
      ],
    });

    await openSelector(user);
    await user.hover(row("gpt-disabled"));
    const favorite = screen.getByTestId("runtime-selector-favorite-action");
    expect(favorite).toBeDisabled();
    expect(favorite).not.toHaveAttribute("aria-keyshortcuts");

    fireEvent.keyDown(screen.getByTestId("runtime-selector-search"), {
      code: "KeyF",
      altKey: true,
    });
    expect(readList(FAVORITES_STORAGE_KEY)).toEqual([]);
  });

  it("Should favorite the highlighted option with the Alt+F keyboard action", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("gpt-a")).toHaveAttribute("data-highlighted", "true"));
    // Alt+F (layout-independent code) — NOT Cmd/Ctrl-D (browser bookmark conflict).
    fireEvent.keyDown(search, { code: "KeyF", altKey: true });

    expect(readList(FAVORITES_STORAGE_KEY)).toContain(runtimeModelKey("codex", "gpt-a"));
  });

  it("Should carry the Alt+F shortcut on the external favorite control, never on listbox options", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    // The option is pure — no aria-keyshortcuts. The real external favorite button
    // is the only carrier of the Alt+F accelerator once it has an active target.
    expect(row("gpt-a")).not.toHaveAttribute("aria-keyshortcuts");
    await user.hover(row("gpt-a"));
    expect(screen.getByTestId("runtime-selector-favorite-action")).toHaveAttribute(
      "aria-keyshortcuts",
      "Alt+F"
    );
  });

  it("Should keep the active (provider,model) target and live aria-activedescendant across favorite reorder", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-one", { name: "GPT One" }), model("gpt-two", { name: "GPT Two" })],
    });

    // Search (no pinned block) so each row renders once; highlight the SECOND row.
    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    await user.type(search, "gpt");
    fireEvent.keyDown(search, { key: "ArrowDown" });
    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("gpt-two")).toHaveAttribute("data-highlighted", "true"));
    expect(optionOrder()).toEqual(["gpt-one", "gpt-two"]);

    // Favorite the active row: favorites rank first, so gpt-two moves to the top.
    fireEvent.keyDown(search, { code: "KeyF", altKey: true });
    await waitFor(() => expect(optionOrder()).toEqual(["gpt-two", "gpt-one"]));

    // The active target followed its (provider,model) identity to the new position,
    // and aria-activedescendant still points at gpt-two — not whatever row now sits
    // at the old index.
    expect(row("gpt-two")).toHaveAttribute("data-highlighted", "true");
    expect(row("gpt-one")).toHaveAttribute("data-highlighted", "false");
    expect(search).toHaveAttribute("aria-activedescendant", row("gpt-two").id);
  });

  it("Should keep pinned and grouped occurrences independently highlightable", async () => {
    const key = runtimeModelKey("codex", "gpt-a");
    window.localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify([key]));
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    const occurrences = document.querySelectorAll<HTMLElement>('[data-model="gpt-a"]');
    expect(occurrences).toHaveLength(2);

    await user.hover(occurrences[1]);
    await waitFor(() => expect(occurrences[1]).toHaveAttribute("data-highlighted", "true"));
    expect(occurrences[0]).toHaveAttribute("data-highlighted", "false");
    const search = screen.getByTestId("runtime-selector-search");
    expect(search).toHaveAttribute("aria-activedescendant", occurrences[1].id);

    await user.click(screen.getByTestId("runtime-selector-favorite-action"));
    await waitFor(() =>
      expect(document.querySelectorAll<HTMLElement>('[data-model="gpt-a"]')).toHaveLength(1)
    );
    const remaining = document.querySelector<HTMLElement>('[data-model="gpt-a"]');
    expect(remaining).toHaveAttribute("data-highlighted", "true");
    expect(search).toHaveAttribute("aria-activedescendant", remaining?.id);
  });

  it("Should announce Favorited/Unfavorited through a polite status while focus stays in search", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("gpt-a")).toHaveAttribute("data-highlighted", "true"));

    const announcer = screen.getByTestId("runtime-selector-announcer");
    expect(announcer).toHaveAttribute("aria-live", "polite");

    fireEvent.keyDown(search, { code: "KeyF", altKey: true });
    await waitFor(() => expect(announcer).toHaveTextContent("Favorited GPT A from Codex"));
    fireEvent.keyDown(search, { code: "KeyF", altKey: true });
    await waitFor(() => expect(announcer).toHaveTextContent("Unfavorited GPT A from Codex"));
  });

  it("Should purge invalid favorites once the catalog loads EMPTY, but not while it is still loading", async () => {
    const ghost = runtimeModelKey("codex", "ghost");
    // Loading (catalogLoaded=false) + empty catalog: nothing is purged — a valid
    // favorite must never be wiped before the catalog resolves.
    window.localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify([ghost]));
    const loading = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [],
      props: { catalogLoaded: false },
    });
    expect(readList(FAVORITES_STORAGE_KEY)).toEqual([ghost]);
    loading.unmount();

    // Loaded-empty (catalogLoaded=true) + empty catalog: every persisted entry is
    // now invalid and is purged so it can never resurrect after reload.
    window.localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify([ghost]));
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [],
      props: { catalogLoaded: true },
    });
    await waitFor(() => expect(readList(FAVORITES_STORAGE_KEY)).toEqual([]));
  });

  it("Should persist picked models to recents deduped and most-recent-first", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" }), model("gpt-b", { name: "GPT B" })],
    });

    await openSelector(user);
    await user.click(row("gpt-a"));
    await user.click(row("gpt-b"));
    await user.click(row("gpt-a"));

    expect(readList(RECENTS_STORAGE_KEY)).toEqual([
      runtimeModelKey("codex", "gpt-a"),
      runtimeModelKey("codex", "gpt-b"),
    ]);
  });

  it("Should cap recents at the configured limit when a new model is picked", async () => {
    // Seed six recents that ARE current tuples (their models exist), so the strict
    // reconcile keeps them and the cap is exercised on valid entries.
    const seededIds = ["r1", "r2", "r3", "r4", "r5", "r6"];
    window.localStorage.setItem(
      RECENTS_STORAGE_KEY,
      JSON.stringify(seededIds.map(id => runtimeModelKey("codex", id)))
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [...seededIds, "r7"].map(id => model(id, { name: id.toUpperCase() })),
    });

    await openSelector(user);
    await user.click(row("r7"));

    const recents = readList(RECENTS_STORAGE_KEY);
    expect(recents).toHaveLength(RECENTS_LIMIT);
    expect(recents[0]).toBe(runtimeModelKey("codex", "r7"));
    expect(recents).not.toContain(runtimeModelKey("codex", "r6"));
  });

  it("Should discard non-current-tuple favorites and re-persist only valid compound keys", async () => {
    const validKey = runtimeModelKey("codex", "gpt-a");
    window.localStorage.setItem(
      FAVORITES_STORAGE_KEY,
      JSON.stringify([
        validKey, // exact current tuple → kept
        "gpt-a", // bare model id (no provider) → dropped
        "garbage-foreign-string", // foreign string → dropped
        runtimeModelKey("codex", "ghost"), // structurally valid but absent from catalog → dropped
      ])
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
    });

    await openSelector(user);

    // Storage is reconciled to ONLY the valid current tuple; the ghosts are purged
    // so they never reappear, and the valid favorite still renders as favorited.
    await waitFor(() => expect(readList(FAVORITES_STORAGE_KEY)).toEqual([validKey]));
    expect(row("gpt-a")).toHaveAttribute("data-favorite", "true");
  });

  it("Should discard non-current-tuple recents and keep each provider's key distinct", async () => {
    const validKey = runtimeModelKey("codex", "gpt-a");
    window.localStorage.setItem(
      RECENTS_STORAGE_KEY,
      JSON.stringify([
        validKey, // codex/gpt-a exists → kept
        "gpt-a", // bare id → dropped
        runtimeModelKey("claude", "gpt-a"), // claude/gpt-a not in catalog → dropped (distinct key)
      ])
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: [codexProvider, claudeProvider],
      models: [model("gpt-a", { provider: "codex", name: "GPT A" })],
    });

    await openSelector(user);

    await waitFor(() => expect(readList(RECENTS_STORAGE_KEY)).toEqual([validKey]));
  });

  it("Should normalize duplicate storage and reject mutations outside the loaded catalog", async () => {
    const validKey = runtimeModelKey("codex", "gpt-a");
    const invalidKey = runtimeModelKey("codex", "ghost");
    window.localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify([validKey, validKey]));
    window.localStorage.setItem(RECENTS_STORAGE_KEY, JSON.stringify([validKey, validKey]));
    const validKeys = new Set([validKey]);
    const { result } = renderHook(() => useRuntimeFavorites(validKeys, true));

    await waitFor(() => expect(readList(FAVORITES_STORAGE_KEY)).toEqual([validKey]));
    await waitFor(() => expect(readList(RECENTS_STORAGE_KEY)).toEqual([validKey]));

    act(() => {
      result.current.toggleFavorite(invalidKey);
      result.current.pushRecent(invalidKey);
    });
    expect(readList(FAVORITES_STORAGE_KEY)).toEqual([validKey]);
    expect(readList(RECENTS_STORAGE_KEY)).toEqual([validKey]);
  });

  it("Should not rewrite unchanged favorites when only the valid-key set identity changes", async () => {
    const validKey = runtimeModelKey("codex", "gpt-a");
    window.localStorage.setItem(FAVORITES_STORAGE_KEY, JSON.stringify([validKey]));
    const writeSpy = vi.spyOn(Storage.prototype, "setItem");
    let validKeys = new Set([validKey]);
    const { rerender } = renderHook(() => useRuntimeFavorites(validKeys, true));

    await waitFor(() => expect(readList(FAVORITES_STORAGE_KEY)).toEqual([validKey]));
    writeSpy.mockClear();
    validKeys = new Set([validKey]);
    rerender();

    expect(writeSpy).not.toHaveBeenCalled();
  });

  it("Should render the pinned 'Recent & favorites' block under the all rail and a 'Favorites' block under the fav rail", async () => {
    window.localStorage.setItem(
      FAVORITES_STORAGE_KEY,
      JSON.stringify([runtimeModelKey("codex", "gpt-a")])
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" }), model("gpt-b", { name: "GPT B" })],
    });

    await openSelector(user);
    const list = screen.getByTestId("runtime-selector-list");
    expect(within(list).getByText("Recent & favorites")).toBeInTheDocument();

    await user.click(document.querySelector('[data-rail="fav"]')!);
    expect(within(list).getByText("Favorites")).toBeInTheDocument();
    expect(within(list).queryByText("Recent & favorites")).not.toBeInTheDocument();
    expect(document.querySelector('[data-model="gpt-a"]')).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Keyboard navigation + segment focus
// ---------------------------------------------------------------------------

describe("RuntimeSelector keyboard navigation", () => {
  const navModels = [
    model("nav-a", { name: "Nav A" }),
    model("nav-b", {
      name: "Nav B",
      disabled: true,
      disabled_reason: "Unavailable",
      availability: "unavailable",
    }),
    model("nav-c", { name: "Nav C" }),
  ];

  it("Should move the highlight with ArrowDown/ArrowUp, skipping disabled rows and wrapping", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: navModels,
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");

    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("nav-a")).toHaveAttribute("data-highlighted", "true"));

    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("nav-c")).toHaveAttribute("data-highlighted", "true"));
    expect(row("nav-b")).toHaveAttribute("data-highlighted", "false");

    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("nav-a")).toHaveAttribute("data-highlighted", "true"));

    fireEvent.keyDown(search, { key: "ArrowUp" });
    await waitFor(() => expect(row("nav-c")).toHaveAttribute("data-highlighted", "true"));
  });

  it("Should select the highlighted row on Enter", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: navModels,
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");
    fireEvent.keyDown(search, { key: "ArrowDown" });
    await waitFor(() => expect(row("nav-a")).toHaveAttribute("data-highlighted", "true"));
    fireEvent.keyDown(search, { key: "Enter" });

    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "nav-a",
      reasoning_effort: "",
    });
  });

  it("Should focus search on open, close on Escape, and restore focus to the trigger", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: navModels,
    });

    await openSelector(user);
    await waitFor(() => expect(screen.getByTestId("runtime-selector-search")).toHaveFocus());
    await user.keyboard("{Escape}");

    await waitFor(() =>
      expect(screen.queryByTestId("runtime-selector-popup")).not.toBeInTheDocument()
    );
    await waitFor(() => expect(screen.getByTestId("rt-trigger")).toHaveFocus());
  });
});

describe("RuntimeSelector listbox semantics", () => {
  it("Should expose loading and empty content as status rather than listbox options", async () => {
    const user = userEvent.setup();
    const loading = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [],
      props: { loading: true },
    });

    await openSelector(user);
    expect(screen.getByTestId("runtime-selector-list")).toHaveAttribute("role", "status");
    expect(screen.queryByRole("listbox", { name: "Models" })).not.toBeInTheDocument();
    expect(screen.getByTestId("runtime-selector-loading")).toBeInTheDocument();
    loading.unmount();

    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [],
      props: { catalogLoaded: true },
    });
    await openSelector(user);
    expect(screen.getByTestId("runtime-selector-list")).toHaveAttribute("role", "status");
    expect(screen.queryByRole("listbox", { name: "Models" })).not.toBeInTheDocument();
    expect(screen.getByTestId("runtime-selector-empty")).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Compound provider·model identity (two providers sharing a model id)
// ---------------------------------------------------------------------------

function rowFor(provider: string, id: string): HTMLElement {
  const el = document.querySelector<HTMLElement>(
    `[data-provider="${provider}"][data-model="${id}"]`
  );
  if (!el) throw new Error(`Missing row ${provider}/${id}`);
  return el;
}

function readStoredList(key: string): string[] {
  const raw = window.localStorage.getItem(key);
  return raw ? (JSON.parse(raw) as string[]) : [];
}

describe("RuntimeSelector compound provider·model identity", () => {
  const twoProviders = [codexProvider, claudeProvider];
  const sharedModels = [
    model("shared-model", { provider: "codex", name: "Shared on Codex", efforts: ["low", "high"] }),
    model("shared-model", {
      provider: "claude",
      name: "Shared on Claude",
      efforts: ["medium", "high"],
    }),
  ];

  it("Should render both providers' rows when a model id is shared across providers", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: sharedModels,
    });

    await openSelector(user);
    expect(rowFor("codex", "shared-model")).toBeInTheDocument();
    expect(rowFor("claude", "shared-model")).toBeInTheDocument();
    expect(document.querySelectorAll('[data-model="shared-model"]')).toHaveLength(2);
  });

  it("Should emit the exact provider of the clicked row for a shared model id", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: sharedModels,
    });

    // Selection keeps the popup open (pick model, then tune reasoning), so both
    // provider rows are clickable within one open session.
    await openSelector(user);
    await user.click(rowFor("claude", "shared-model"));
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "claude",
      model: "shared-model",
      reasoning_effort: "",
    });

    await user.click(rowFor("codex", "shared-model"));
    expect(onChange).toHaveBeenLastCalledWith({
      provider: "codex",
      model: "shared-model",
      reasoning_effort: "",
    });
  });

  it("Should favorite only the active provider's row for a shared model id", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: sharedModels,
    });

    await openSelector(user);
    // Hover the Claude row so the external favorite action targets that exact tuple.
    await user.hover(rowFor("claude", "shared-model"));
    await user.click(screen.getByTestId("runtime-selector-favorite-action"));

    const favorites = readStoredList(FAVORITES_STORAGE_KEY);
    expect(favorites).toContain(runtimeModelKey("claude", "shared-model"));
    expect(favorites).not.toContain(runtimeModelKey("codex", "shared-model"));
    // Compound identity: only the Claude row flips; the Codex row stays un-favorited.
    expect(rowFor("claude", "shared-model")).toHaveAttribute("data-favorite", "true");
    expect(rowFor("codex", "shared-model")).toHaveAttribute("data-favorite", "false");
  });
});

// ---------------------------------------------------------------------------
// Provider rail — local filter only, cross-provider favorites
// ---------------------------------------------------------------------------

describe("RuntimeSelector provider rail filtering", () => {
  const twoProviders = [codexProvider, claudeProvider];
  const railModels = [
    model("codex-a", { provider: "codex", name: "Codex A" }),
    model("claude-a", { provider: "claude", name: "Claude A" }),
  ];

  it("Should filter the list to a provider without emitting a value change", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "codex-a", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
    });

    await openSelector(user);
    await user.click(document.querySelector('[data-rail="claude"]')!);

    await waitFor(() => expect(rowFor("claude", "claude-a")).toBeInTheDocument());
    expect(
      document.querySelector('[data-provider="codex"][data-model="codex-a"]')
    ).not.toBeInTheDocument();
    // Rail is a list filter: the controlled value never changes.
    expect(onChange).not.toHaveBeenCalled();
  });

  it("Should surface favorites from every provider under the Favorites rail", async () => {
    window.localStorage.setItem(
      FAVORITES_STORAGE_KEY,
      JSON.stringify([runtimeModelKey("codex", "codex-a"), runtimeModelKey("claude", "claude-a")])
    );
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
    });

    await openSelector(user);
    await user.click(document.querySelector('[data-rail="fav"]')!);

    await waitFor(() => {
      expect(rowFor("codex", "codex-a")).toBeInTheDocument();
      expect(rowFor("claude", "claude-a")).toBeInTheDocument();
    });
  });

  it("Should be a radiogroup (not tabs) and move the filter with roving arrow-key navigation", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
    });

    await openSelector(user);
    // Truthful semantics: a local mutually-exclusive filter is a radiogroup, never
    // a tablist (the rail does not swap a tabpanel — it filters the list in place).
    const radiogroup = document.querySelector<HTMLElement>('[role="radiogroup"]')!;
    expect(radiogroup).toBeInTheDocument();
    expect(document.querySelector('[role="tablist"]')).toBeNull();
    expect(document.querySelector('[data-rail="all"]')).toHaveAttribute("role", "radio");
    expect(document.querySelector('[data-rail="all"]')).toHaveAttribute("aria-checked", "true");
    // The checked radio (All) is the only one in the Tab sequence (roving tabindex).
    expect(document.querySelector('[data-rail="all"]')).toHaveAttribute("tabindex", "0");
    expect(document.querySelector('[data-rail="claude"]')).toHaveAttribute("tabindex", "-1");

    fireEvent.keyDown(radiogroup, { key: "ArrowDown" });
    await waitFor(() =>
      expect(document.querySelector('[data-rail="fav"]')).toHaveAttribute("data-active", "true")
    );
  });

  it("Should keep Provider Settings structurally outside the filter radiogroup", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
      props: { onOpenProviderSettings: vi.fn() },
    });

    await openSelector(user);
    const settings = screen.getByTestId("runtime-selector-settings");
    const radiogroup = document.querySelector('[role="radiogroup"]')!;
    // Settings is an escape hatch, not a fourth filter — it must not live in the group.
    expect(radiogroup.contains(settings)).toBe(false);
    expect(settings).not.toHaveAttribute("role", "radio");
  });

  it("Should jump the rail filter to the last provider on End and back to All on Home", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
    });

    await openSelector(user);
    const radiogroup = document.querySelector<HTMLElement>('[role="radiogroup"]')!;

    fireEvent.keyDown(radiogroup, { key: "End" });
    await waitFor(() =>
      expect(document.querySelector('[data-rail="claude"]')).toHaveAttribute("data-active", "true")
    );

    fireEvent.keyDown(radiogroup, { key: "Home" });
    await waitFor(() =>
      expect(document.querySelector('[data-rail="all"]')).toHaveAttribute("data-active", "true")
    );
  });

  it("Should suppress rail radios while searching (not keyboard-activatable)", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      providers: twoProviders,
      models: railModels,
    });

    await openSelector(user);
    fireEvent.change(screen.getByTestId("runtime-selector-search"), { target: { value: "codex" } });

    await waitFor(() => expect(document.querySelector('[data-rail="codex"]')).toBeDisabled());
    expect(document.querySelector('[data-rail="all"]')).toBeDisabled();
  });
});

// ---------------------------------------------------------------------------
// Row chips + reasoning footer modes
// ---------------------------------------------------------------------------

describe("RuntimeSelector single-line row", () => {
  it("Should render no metadata chips — the name is the only visible row text", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [
        model("rich", {
          name: "Rich",
          context_window: 1_050_000,
          cost_input: 5,
          cost_output: 30,
          cost_cache_read: 0.5,
          cost_cache_write: 8,
          cost_reasoning: 40,
          supports_tools: true,
          efforts: ["low", "medium", "high"],
        }),
      ],
    });

    await openSelector(user);
    const richRow = row("rich");
    // Even a metadata-rich model renders a single line: no context window,
    // no cost rates, no tools chip, no levels count.
    expect(richRow).not.toHaveTextContent("1.05M");
    expect(richRow).not.toHaveTextContent("$");
    expect(richRow).not.toHaveTextContent("tools");
    expect(richRow).not.toHaveTextContent("levels");
  });

  it("Should mark reasoning-capable rows with the brain indicator", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [
        model("leveled", { name: "Leveled", efforts: ["low", "high"] }),
        model("supp", { name: "Supported", supports_reasoning: true, efforts: [] }),
        model("plain", { name: "Plain", efforts: [] }),
      ],
    });

    await openSelector(user);
    // Both selectable-levels and supports-without-levels models carry the
    // indicator; a non-reasoning model does not.
    expect(row("leveled").querySelector("[data-reasoning-indicator]")).not.toBeNull();
    expect(row("supp").querySelector("[data-reasoning-indicator]")).not.toBeNull();
    expect(row("plain").querySelector("[data-reasoning-indicator]")).toBeNull();
  });

  it("Should mark the selected row with a neutral tint and a check, never the accent tint", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "picked", reasoning_effort: "" },
      models: [model("picked", { name: "Picked" }), model("other", { name: "Other" })],
    });

    await openSelector(user);
    const selected = document.querySelector<HTMLElement>(
      '[data-model="picked"][data-selected="true"]'
    );
    expect(selected).not.toBeNull();
    // Selection is the neutral row tint + structural check — not the accent wash.
    expect(selected?.className).toContain("bg-row-selected");
    expect(selected?.className).not.toContain("bg-accent-tint");
    expect(selected?.querySelector("[data-selected-check]")).not.toBeNull();
  });
});

describe("RuntimeSelector reasoning footer modes", () => {
  async function footerFor(models: RuntimeModelOption[], value: RuntimeSelectorValue) {
    const user = userEvent.setup();
    renderSelector({ value, models });
    await openSelector(user);
    return screen.getByTestId("runtime-selector-reasoning");
  }

  it("Should prompt to pick a model when nothing is selected (no 'this model' claim)", async () => {
    const footer = await footerFor([model("a")], {
      provider: "codex",
      model: "",
      reasoning_effort: "",
    });
    expect(footer).toHaveAttribute("data-reasoning-mode", "no-model");
    expect(footer).toHaveTextContent("Select a model");
  });

  it("Should show the 'provider decides' note for supported-without-levels models", async () => {
    const footer = await footerFor([model("supp", { supports_reasoning: true, efforts: [] })], {
      provider: "codex",
      model: "supp",
      reasoning_effort: "",
    });
    expect(footer).toHaveAttribute("data-reasoning-mode", "supported-nolevels");
    expect(footer).toHaveTextContent("Reasoning is on");
  });

  it("Should show the 'no reasoning effort' note for models without reasoning", async () => {
    const footer = await footerFor([model("plain", { supports_reasoning: false, efforts: [] })], {
      provider: "codex",
      model: "plain",
      reasoning_effort: "",
    });
    expect(footer).toHaveAttribute("data-reasoning-mode", "none");
    expect(footer).toHaveTextContent("doesn’t use reasoning effort");
  });
});

// ---------------------------------------------------------------------------
// Reasoning strip label identity — per-instance ARIA ids
// ---------------------------------------------------------------------------

describe("RuntimeSelector reasoning slider label identity", () => {
  it("Should give each mounted reasoning slider a unique, locally-wired label id", () => {
    const reasoning = resolveReasoningState(
      model("leveled", { efforts: ["low", "high"], reasoning_source: "catalog" })
    );
    render(
      <UIProvider reducedMotion="always">
        <ReasoningBar
          reasoning={reasoning}
          value=""
          providerName="Codex"
          modelName="Leveled"
          onSelect={() => {}}
        />
        <ReasoningBar
          reasoning={reasoning}
          value=""
          providerName="Codex"
          modelName="Leveled"
          onSelect={() => {}}
        />
      </UIProvider>
    );

    const labelledby = screen
      .getAllByRole("slider")
      .map(slider => slider.getAttribute("aria-labelledby"));
    expect(labelledby).toHaveLength(2);
    // Distinct per-instance ids (no shared fixed id colliding across mounts).
    expect(labelledby[0]).toBeTruthy();
    expect(labelledby[0]).not.toBe(labelledby[1]);
    // Each slider points at a label that exists locally in the document.
    for (const id of labelledby) {
      expect(document.getElementById(id ?? "")).toBeInTheDocument();
    }
  });
});

// ---------------------------------------------------------------------------
// No global shortcut — the selector opens only through its trigger
// ---------------------------------------------------------------------------

describe("RuntimeSelector shortcut removal", () => {
  it("Should not open on ⌘J from the composer scope — the trigger is the only opener", () => {
    function Instance({ testId }: { testId: string }) {
      const [value, setValue] = useState<RuntimeSelectorValue>({
        provider: "codex",
        model: "",
        reasoning_effort: "",
      });
      return (
        <RuntimeSelector
          value={value}
          onChange={setValue}
          providers={[codexProvider]}
          models={[]}
          triggerTestId={testId}
        />
      );
    }
    render(
      <UIProvider reducedMotion="always">
        <form>
          <input aria-label="Prompt" data-testid="composer-input" />
          <Instance testId="rt-a" />
        </form>
      </UIProvider>
    );

    const composerInput = screen.getByTestId("composer-input");
    composerInput.focus();
    fireEvent.keyDown(composerInput, { key: "j", metaKey: true });
    expect(screen.queryAllByTestId("runtime-selector-popup")).toHaveLength(0);
    expect(screen.getByTestId("rt-a")).toHaveAttribute("data-open", "false");
  });
});

// ---------------------------------------------------------------------------
// Home/End highlight + Provider Settings order
// ---------------------------------------------------------------------------

describe("RuntimeSelector list keyboard edges", () => {
  it("Should jump the highlight to the last row on End and the first row on Home", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("edge-a"), model("edge-b"), model("edge-c")],
    });

    await openSelector(user);
    const search = screen.getByTestId("runtime-selector-search");

    fireEvent.keyDown(search, { key: "End" });
    await waitFor(() => expect(row("edge-c")).toHaveAttribute("data-highlighted", "true"));

    fireEvent.keyDown(search, { key: "Home" });
    await waitFor(() => expect(row("edge-a")).toHaveAttribute("data-highlighted", "true"));
  });
});

describe("RuntimeSelector provider settings action order", () => {
  it("Should close the popup then invoke onOpenProviderSettings when the gear is clicked", async () => {
    const user = userEvent.setup();
    const onOpenProviderSettings = vi.fn();
    renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a", { name: "GPT A" })],
      props: { onOpenProviderSettings },
    });

    await openSelector(user);
    await user.click(screen.getByTestId("runtime-selector-settings"));

    expect(onOpenProviderSettings).toHaveBeenCalledTimes(1);
    await waitFor(() =>
      expect(screen.queryByTestId("runtime-selector-popup")).not.toBeInTheDocument()
    );
  });
});

// ---------------------------------------------------------------------------
// Intensity meter
// ---------------------------------------------------------------------------

describe("IntensityMeter + reasoningEffortPosition", () => {
  it("Should map each canonical effort to its 1-based position", () => {
    expect(REASONING_EFFORT_ORDER.map(reasoningEffortPosition)).toEqual([1, 2, 3, 4, 5, 6, 7]);
    expect(reasoningEffortPosition("")).toBe(0);
    expect(reasoningEffortPosition("nonsense")).toBe(0);
  });

  it("Should mark exactly the requested number of bars as filled for each canonical position", () => {
    for (let position = 0; position <= 7; position += 1) {
      const { container, unmount } = render(<IntensityMeter position={position} />);
      const meter = container.querySelector('[data-slot="intensity-meter"]');
      const bars = Array.from(meter?.children ?? []);
      expect(bars).toHaveLength(7);
      expect(meter).toHaveAttribute("data-position", String(position));
      expect(bars.filter(bar => bar.getAttribute("data-fill") === "on")).toHaveLength(position);
      unmount();
    }
  });

  it("Should mark every bar hollow with zero filled when hollow", () => {
    const { container } = render(<IntensityMeter position={5} hollow />);
    const meter = container.querySelector('[data-slot="intensity-meter"]');
    const bars = Array.from(meter?.children ?? []);
    expect(meter).toHaveAttribute("data-hollow", "true");
    expect(meter).toHaveAttribute("data-position", "0");
    expect(bars.filter(bar => bar.getAttribute("data-fill") === "on")).toHaveLength(0);
    expect(bars.every(bar => bar.getAttribute("data-fill") === "hollow")).toBe(true);
  });
});

describe("RuntimeSelector composer variant", () => {
  const composerValue: RuntimeSelectorValue = {
    provider: "codex",
    model: "gpt-a",
    reasoning_effort: "high",
  };
  const composerModels = [model("gpt-a", { efforts: ["low", "medium", "high"] })];

  it("Should keep the full runtime identity in the composer", () => {
    renderSelector({
      value: composerValue,
      models: composerModels,
      props: { variant: "composer" },
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger).toHaveTextContent("gpt-a");
    expect(trigger).toHaveAccessibleName(/Codex \/ gpt-a, reasoning High/);
  });

  it("Should open and close the popup exactly like the default variant", async () => {
    const user = userEvent.setup();
    renderSelector({
      value: composerValue,
      models: composerModels,
      props: { variant: "composer" },
    });

    const trigger = screen.getByTestId("rt-trigger");
    expect(trigger).toHaveAttribute("aria-expanded", "false");

    await openSelector(user);
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByTestId("runtime-selector-popup")).toBeNull());
    expect(trigger).toHaveFocus();
  });

  it("Should still commit a model selection through onChange", async () => {
    const user = userEvent.setup();
    const { onChange } = renderSelector({
      value: { provider: "codex", model: "", reasoning_effort: "" },
      models: [model("gpt-a"), model("gpt-b")],
      props: { variant: "composer" },
    });

    await openSelector(user);
    await user.click(row("gpt-b"));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ model: "gpt-b" }));
  });
});
