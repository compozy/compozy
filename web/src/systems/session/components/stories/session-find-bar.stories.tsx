import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { primarySessionFixture } from "@/systems/session/mocks";

import { useSessionFind } from "../../hooks/use-session-navigation";
import type { SessionFindJumpHandlers } from "../../lib/session-navigation-find-store";
import type { SessionTranscriptSearchResponse } from "../../types";
import { SessionFindBar, type SessionFindBarProps } from "../session-find-bar";

const SEARCH_ROUTE = "/api/workspaces/{workspace_id}/sessions/{session_id}/transcript/search";

const LIFECYCLE_MATCHES: SessionTranscriptSearchResponse = {
  matches: [
    {
      role: "user",
      sequence: 12,
      snippet: "Refactor the flaky manager tests — the lifecycle ones first",
      turn_id: "turn-1",
    },
    {
      role: "tool",
      sequence: 31,
      snippet: "go test ./internal/session/... -run Lifecycle -count=5",
      turn_id: "turn-2",
    },
    {
      role: "assistant",
      sequence: 44,
      snippet: "…waits on the lifecycle channel instead",
      turn_id: "turn-2",
    },
    {
      role: "assistant",
      sequence: 58,
      snippet: "…retry path in manager_lifecycle_test.go no longer depends on wall-clock sleeps",
      turn_id: "turn-3",
    },
    {
      role: "user",
      sequence: 90,
      snippet: "Only touch the lifecycle tests, skip the store package",
      turn_id: "turn-4",
    },
  ],
  truncated: false,
};

function searchHandler(respond: (query: string) => SessionTranscriptSearchResponse) {
  return storybookMswParameters({
    session: [
      compozyApiMock.get(SEARCH_ROUTE, ({ request }) =>
        HttpResponse.json(respond(new URL(request.url).searchParams.get("q") ?? ""))
      ),
    ],
  });
}

// The viewport host owns the `useSessionFind` model; the story stands the host
// in with the same scope, seed and jump handlers.
type FindBarStoryProps = SessionFindJumpHandlers &
  Omit<SessionFindBarProps, "find"> & {
    workspaceId: string;
    sessionId: string;
    initialQuery?: string;
  };

function FindBarStory({
  workspaceId,
  sessionId,
  initialQuery,
  isSequenceLoaded,
  loadOlderUntil,
  jumpToSequence,
  ...bar
}: FindBarStoryProps) {
  const find = useSessionFind({
    handlers: { isSequenceLoaded, jumpToSequence, loadOlderUntil },
    initialQuery,
    open: true,
    sessionId,
    workspaceId,
  });
  return <SessionFindBar {...bar} find={find} />;
}

/**
 * Find in conversation (S8): the bar docks under the window head; the field
 * queries the daemon's full-history search. The host owns jumping (fold-open
 * + scroll ownership); stories stand those callbacks in.
 */
const meta: Meta<typeof FindBarStory> = {
  title: "systems/session/components/SessionFindBar",
  component: FindBarStory,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full-history find: SearchInput + count slot (keys · N of M · No matches · Loading older… · +N new) + prev/next + close, and a Command-row list of matches with the hit glazed and a `folded` tag when it sits behind a settled fold. Enter/Shift+Enter step, ↑/↓ move the selection, Esc closes.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-3xl overflow-hidden rounded-lg border border-line bg-canvas">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    agentName: primarySessionFixture.agent_name,
    isSequenceLoaded: (sequence: number) => sequence >= 40,
    jumpToSequence: fn(),
    loadOlderUntil: fn(async () => true),
    onClose: fn(),
    sessionId: primarySessionFixture.id,
    workspaceId: primarySessionFixture.workspace_id!,
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-02 (open-empty) — the field alone with the key hints until the first character. */
export const OpenEmpty: Story = {
  parameters: searchHandler(() => ({ matches: [], truncated: false })),
};

/** VC-01 — matches with the list, the tool row behind a fold, the first two in unloaded history. */
export const Matches: Story = {
  args: { initialQuery: "lifecycle", isSequenceFolded: (sequence: number) => sequence === 31 },
  parameters: searchHandler(() => LIFECYCLE_MATCHES),
};

/** VC-02 — nothing found, said plainly, with no step controls. */
export const NoMatches: Story = {
  args: { initialQuery: "sqlite" },
  parameters: searchHandler(() => ({ matches: [], truncated: false })),
};

/** The daemon capped the list: the count reads `200+` and the list says how to see the rest. */
export const Truncated: Story = {
  args: { initialQuery: "the" },
  parameters: searchHandler(() => ({ ...LIFECYCLE_MATCHES, truncated: true })),
};

/** The search route failed: the count slot says so and offers Try again. */
export const SearchFailed: Story = {
  args: { initialQuery: "lifecycle" },
  parameters: storybookMswParameters({
    session: [
      compozyApiMock.get(SEARCH_ROUTE, () =>
        HttpResponse.json({ error: "transcript search unavailable" }, { status: 503 })
      ),
    ],
  }),
};
