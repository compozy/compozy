import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { primarySessionFixture } from "@/systems/session/mocks";

import type { SessionTranscriptOutlineResponse } from "../../types";
import { SessionMessageTrail } from "../session-message-trail";

const OUTLINE_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/outline";

const ASKS = [
  "Refactor the flaky manager tests — the lifecycle ones first",
  "Only touch the lifecycle tests, skip the store package",
  "Now make the same change in the store package",
  "Ship it with tests",
  "Also update the changelog",
  "Open a draft PR",
];

function outline(count: number): SessionTranscriptOutlineResponse {
  return {
    entries: Array.from({ length: count }, (_, index) => ({
      at: new Date(Date.UTC(2026, 8, 6, 14, index % 60)).toISOString(),
      preview: ASKS[index % ASKS.length]!,
      reply_preview:
        index === count - 1
          ? ""
          : "Understood — the store package is untouched. The lifecycle tests now wait on the manager's settle signal instead of sleeping; 14 updated, all green with -count=5.",
      sequence: (index + 1) * 10,
      turn_id: `turn-${index + 1}`,
    })),
  };
}

function outlineHandler(count: number) {
  return storybookMswParameters({
    session: [compozyApiMock.get(OUTLINE_ROUTE, () => HttpResponse.json(outline(count)))],
  });
}

/**
 * The message trail (S9): a 28px rail with one tick per sent message from the
 * daemon's outline. Layout depends only on the count and the pane height;
 * hover/focus shows what was asked and the final reply; the host owns the jump.
 */
const meta: Meta<typeof SessionMessageTrail> = {
  title: "systems/session/components/SessionMessageTrail",
  component: SessionMessageTrail,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One 2px tick per operator message: rest ticks faint, in-view stronger, the reading anchor wider and `aria-current`; width breathes toward the pointer like a dock. Spacing shrinks with the count and past the pane share the ticks spread proportionally. One roving tab stop.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="relative flex h-80 w-full max-w-3xl rounded-lg border border-line bg-canvas p-3">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    onJumpToSequence: fn(),
    paneHeightPx: 300,
    sessionId: primarySessionFixture.id,
    viewportTopSequence: 25,
    visibleRange: { from: 20, to: 45 },
    workspaceId: primarySessionFixture.workspace_id!,
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-04 — six sent messages: anchor on the second, the third and fourth in view. */
export const FewTicks: Story = {
  parameters: outlineHandler(6),
};

/** Twenty-four messages: the gap has shrunk, nothing is compressed yet. */
export const ManyTicks: Story = {
  parameters: outlineHandler(24),
};

/** VC-05 — sixty messages spread proportionally across the pane share; hover to magnify. */
export const Dense: Story = {
  args: { paneHeightPx: 220 },
  parameters: outlineHandler(60),
};
