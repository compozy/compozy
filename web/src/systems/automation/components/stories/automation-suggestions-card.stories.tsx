import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { PanelSurface } from "@/storybook/story-layout";
import type { AutomationSuggestion } from "../../types";

import { AutomationSuggestionsCard } from "../automation-suggestions-card";

const suggestion: AutomationSuggestion = {
  created_at: "2026-07-18T12:00:00Z",
  dedup_key: "daily-review",
  id: "suggestion_daily_review",
  payload: {
    agent_name: "reviewer",
    created_at: "2026-07-18T12:00:00Z",
    enabled: true,
    fire_limit: { max: 12, window: "1h" },
    id: "job_daily_review",
    name: "Daily review",
    prompt: "Review changes merged since the previous run and summarize any release risk.",
    retry: { base_delay: "2s", max_retries: 3, strategy: "none" },
    schedule: { expr: "0 9 * * 1-5", mode: "cron" },
    scope: "workspace",
    source: "dynamic",
    target_kind: "prompt",
    updated_at: "2026-07-18T12:00:00Z",
    workspace_id: "ws_alpha",
  },
  source: "catalog",
  status: "pending",
  workspace_id: "ws_alpha",
};

const meta: Meta<typeof AutomationSuggestionsCard> = {
  component: AutomationSuggestionsCard,
  title: "systems/automation/components/AutomationSuggestionsCard",
  args: {
    onAccept: fn(),
    onDismiss: fn(),
    onRetry: fn(),
    suggestions: [suggestion],
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-72 min-w-0">
        <Story />
      </PanelSurface>
    ),
  ],
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Loading: Story = {
  args: { isLoading: true, suggestions: [] },
};

export const QueryError: Story = {
  args: {
    errorMessage: "Couldn't load automation suggestions. Retry the request.",
    suggestions: [],
  },
};

export const ActionError: Story = {
  args: {
    actionErrors: {
      [suggestion.id]: "Couldn't create this job. Review the proposal and try again.",
    },
  },
};

export const ActionPending: Story = {
  args: { pendingActions: { [suggestion.id]: "accept" } },
};
