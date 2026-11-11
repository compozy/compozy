import type {
  RuntimeACPOption,
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorProps,
  RuntimeSelectorValue,
} from "../runtime-selector";

// Truthful aggregate fixtures: two live providers plus a signed-out one, and a
// cross-provider model set carrying real curation / availability / reasoning
// metadata (a shared canonical id `gpt-5.6-sol` proves compound identity).
export const runtimeSelectorStoryProviders: RuntimeProviderOption[] = [
  { id: "codex", name: "Codex", runtime_provider: "codex", harness: "acp" },
  { id: "claude", name: "Claude", runtime_provider: "claude", harness: "acp" },
  { id: "openrouter", name: "OpenRouter", runtime_provider: "openrouter", harness: "pi_acp" },
  { id: "cursor", name: "Cursor", runtime_provider: "cursor", harness: "acp", needs_auth: true },
];

function runtimeModel(overrides: Partial<RuntimeModelOption> & { id: string; provider: string }) {
  return {
    name: overrides.id,
    efforts: [],
    availability: "live",
    curated: true,
    supports_tools: true,
    ...overrides,
  } satisfies RuntimeModelOption;
}

export const runtimeSelectorStoryModels: RuntimeModelOption[] = [
  runtimeModel({
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
  runtimeModel({
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
  runtimeModel({
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
  runtimeModel({
    id: "claude-haiku-4-5-20251001",
    provider: "claude",
    name: "Claude Haiku 4.5",
    context_window: 200_000,
    cost_input: 1,
    cost_output: 5,
    // Canonical builtin metadata: supports_reasoning with no selectable effort
    // subset means the trigger drops its reasoning segment and the footer reads
    // "provider decides" (internal/config/provider_reasoning.go).
    supports_reasoning: true,
  }),
  runtimeModel({
    id: "gpt-5.4-mini",
    provider: "codex",
    name: "GPT-5.4 Mini",
    // Non-curated: hidden under All models browsing, revealed by a provider chip or search.
    curated: false,
    availability: "stale",
  }),
  // Same canonical id under a different provider is a distinct,
  // independently selectable and favoritable row.
  runtimeModel({
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

// Deliberately long display name so the truncation + hover-reveal contract is
// reviewable in Storybook.
export const runtimeSelectorStoryLongNameModel: RuntimeModelOption = runtimeModel({
  id: "openrouter/anthropic/claude-sonnet-4.6-long-context-preview",
  provider: "opencode",
  name: "Claude Sonnet 4.6 Long Context Preview",
});

export const runtimeSelectorStoryAdvancedOptions: RuntimeACPOption[] = [
  {
    id: "context_window",
    label: "Context window",
    description: "Maximum context for the next run",
    kind: "select",
    current_value_id: "200k",
    values: [
      {
        value: "200k",
        label: "200k tokens",
        group_id: "standard",
        group_label: "Standard",
      },
      {
        value: "1m",
        label: "1M tokens",
        group_id: "large",
        group_label: "Large",
      },
    ],
  },
  {
    id: "thinking",
    label: "Thinking",
    description: "Allow the provider to spend more time reasoning",
    kind: "boolean",
    current_bool: false,
  },
];

/** One closed-state cell in the trigger contract matrix (see the `Default` story). */
export interface RuntimeSelectorStoryClosedState {
  label: string;
  description: string;
  value: RuntimeSelectorValue;
  variant?: RuntimeSelectorProps["variant"];
  disabled?: boolean;
}

/**
 * Every closed-state contract in one deterministic canvas: default, compact,
 * small, composer (idle and inert), no selectable reasoning, and provider
 * authentication needed.
 */
export const runtimeSelectorStoryClosedStates: RuntimeSelectorStoryClosedState[] = [
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

// OpenCode is absent from the base set: the long-name story adds it to prove
// provider-label truncation next to the model-name truncation.
export const runtimeSelectorStoryOpenCodeProvider: RuntimeProviderOption = {
  id: "opencode",
  name: "OpenCode",
  runtime_provider: "opencode",
};

// More providers than the 320px rail can show, so the story proves the paging
// affordance rather than a fixture that happens to fit.
export const runtimeSelectorStoryOverflowProviders: RuntimeProviderOption[] = [
  ...runtimeSelectorStoryProviders,
  ...Array.from({ length: 8 }, (_, index) => ({
    id: `rail-${index}`,
    name: `Rail provider ${index}`,
    runtime_provider: "codex",
  })),
];

export const runtimeSelectorStoryManyIssueProviders: RuntimeProviderOption[] = [
  ...runtimeSelectorStoryProviders,
  ...Array.from({ length: 12 }, (_, index) => ({
    id: `offline-${index}`,
    name: `Offline provider ${index}`,
    runtime_provider: "codex",
  })),
];

export const runtimeSelectorStoryManyProviderIssues: Record<string, string> = Object.fromEntries([
  ["cursor", "Sign in to update the model list."],
  ...Array.from({ length: 12 }, (_, index) => [
    `offline-${index}`,
    "Could not update the model list. Try refreshing again.",
  ]),
]);
