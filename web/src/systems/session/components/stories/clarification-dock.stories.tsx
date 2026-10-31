import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

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

const multilineClarification: ClarificationPending = {
  agent_name: "release-captain",
  session_id: "sess-001",
  request_id: "req-repo-confirm",
  question: [
    "Create the repository with these settings?",
    "",
    "Repository",
    "- Name: payments-api",
    "- Teams: platform, billing",
    "- Visibility: internal",
    "",
    "Jira Assets — Application",
    "- Owner: platform-team",
    "- Tier: 1 (customer facing)",
    "- Data class: confidential",
    "",
    "CI",
    "- Required checks: lint, test, e2e",
    "- Deploy target: staging, then production",
    "",
    "Confirm?",
  ].join("\n"),
  choices: ["Yes, create it", "Change settings", "Cancel"],
  asked_at: "2026-07-19T20:00:00Z",
  deadline: "2026-07-19T20:05:00Z",
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
  // The dock is a size container (its head reflows below 28rem), so the frame
  // gives it a definite width inside the centered story layout.
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-xl max-w-[calc(100vw-3rem)]">
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

/** The agent's line breaks survive; the question scrolls inside a dock bounded to half the viewport. */
export const MultiLine: Story = {
  args: { clarification: multilineClarification, countLabel: "1/2" },
};

/** Phone viewport: the bounded dock leaves the transcript visible above it. */
export const MultiLineMobile: Story = {
  args: { clarification: multilineClarification },
  parameters: { viewport: { defaultViewport: "iphone14" } },
};

/** The chevron collapses the dock to the question's first line; digit keys still answer. */
export const Collapsed: Story = {
  args: { clarification: multilineClarification, countLabel: "1/2" },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const toggle = await canvas.findByTestId("clarification-dock-toggle");
    await userEvent.click(toggle);
    await expect(toggle).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.getByTestId("clarification-dock-question")).toHaveTextContent(
      /^Create the repository with these settings\?$/
    );
    await expect(canvas.queryByTestId("clarification-dock-choices")).toBeNull();
  },
};
