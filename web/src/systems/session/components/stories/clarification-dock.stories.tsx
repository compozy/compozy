import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { ClarificationPending } from "@/systems/session/types";

import { ClarificationDock } from "../clarification-dock";

const boundedClarification: ClarificationPending = {
  agent_name: "release-captain",
  session_id: "sess-001",
  request_id: "req-release-path",
  question: "Which release path should the payout migration use?",
  choices: ["Canary", "Immediate", "Hold for review"],
  asked_at: "2026-07-19T20:00:00Z",
  deadline: "2026-07-19T20:05:00Z",
};

const freeTextClarification: ClarificationPending = {
  agent_name: "release-captain",
  session_id: "sess-001",
  request_id: "req-release-name",
  question: "What should the release be called?",
  choices: [],
  asked_at: "2026-07-19T20:00:00Z",
  deadline: "2026-07-19T20:10:00Z",
};

const meta: Meta<typeof ClarificationDock> = {
  title: "systems/session/components/ClarificationDock",
  component: ClarificationDock,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Pending clarification as a composer-docked decision panel. Bounded questions answer as 30px rows on keys 1–9; a question with no choices renders the quiet free-text form (Enter submits, Shift+Enter breaks). The deadline hint is static mono — it never ticks.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    sessionId: "sess-001",
    workspaceId: "ws_alpha",
    countLabel: null,
    onResolved: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const BoundedChoices: Story = {
  args: { clarification: boundedClarification },
};

export const FreeText: Story = {
  args: { clarification: freeTextClarification },
};

export const Queued: Story = {
  args: { clarification: boundedClarification, countLabel: "2/3" },
};
