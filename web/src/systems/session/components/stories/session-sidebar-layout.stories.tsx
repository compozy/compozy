import type { Meta, StoryObj } from "@storybook/react-vite";
import { MessagesSquare } from "lucide-react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { Empty } from "@compozy/ui";

import { CenteredSurface } from "@/storybook/story-layout";

import { sessionSidebarStore } from "../../hooks/use-session-sidebar-state";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "../../lib/session-sidebar-width";
import { SessionSidebar } from "../session-sidebar";
import { SessionSidebarLayout } from "../session-sidebar-layout";
import { PROVENANCE_FAMILY } from "./session-sidebar-fixtures";

type StoryArgs = { open: boolean; width: number | null; frameWidth: number };

/** Seed the shared rail preference before mount; restore the default after. */
function seedRail({ open, width }: Pick<StoryArgs, "open" | "width">) {
  return () => {
    sessionSidebarStore.trigger.sidebarVisibilityChanged({ open });
    if (width === null) {
      sessionSidebarStore.trigger.widthReset();
    } else {
      sessionSidebarStore.trigger.widthChanged({ width });
    }
    return () => {
      sessionSidebarStore.trigger.sidebarVisibilityChanged({ open: true });
      sessionSidebarStore.trigger.widthReset();
    };
  };
}

function railWidth(canvasElement: HTMLElement): number {
  return within(canvasElement).getByTestId("session-sidebar").getBoundingClientRect().width;
}

const meta: Meta<StoryArgs> = {
  title: "systems/session/components/SessionSidebarLayout",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Session window body: the sessions rail and the session pane split by a resize handle. Drag or use the arrow keys on the handle to resize; double-click resets to the default width. The width is one preference shared by every session window and holds in pixels when the window resizes, between 200 and 480 px and never past half the window.",
      },
    },
  },
  args: { open: true, width: null, frameWidth: 960 },
  render: ({ open, frameWidth }) => (
    <CenteredSurface>
      <div
        className="flex h-[560px] overflow-hidden rounded-lg border border-line-strong bg-canvas"
        style={{ width: frameWidth }}
      >
        <SessionSidebarLayout
          sidebar={
            <SessionSidebar
              open={open}
              sessions={PROVENANCE_FAMILY}
              disconnected={false}
              collapsedThreadIds={[]}
              currentSessionId="sess-migration"
              view={{
                scope: "workspace",
                sort: "last_activity",
                archived: false,
                saving: false,
                setScope: fn(),
                setSort: fn(),
                setArchived: fn(),
                workspaceGroups: [],
                collapsedWorkspaceIds: new Set(),
                toggleWorkspace: fn(),
                aggregate: false,
                scopeLabel: "default",
                ownerOf: () => ({ id: "default", name: "default", archived: false }),
              }}
              onToggleThread={fn()}
              onSelectSession={fn()}
              onNewSession={fn()}
              sessionActions={{
                pendingAction: null,
                pendingSessionId: null,
                onArchive: fn(),
                onDelete: fn(),
                onRename: fn(),
                onStop: fn(),
                onUnarchive: fn(),
              }}
            />
          }
        >
          <div className="flex min-w-0 flex-1 items-center justify-center">
            <Empty
              description="Pick one from the list, or start a new one."
              icon={MessagesSquare}
              title="No session selected"
            />
          </div>
        </SessionSidebarLayout>
      </div>
    </CenteredSurface>
  ),
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Calm default: the rail opens at its historical width. */
export const DefaultWidth: Story = {
  beforeEach: seedRail({ open: true, width: null }),
};

/** A width the operator chose earlier is restored on mount. */
export const PersistedWidth: Story = {
  args: { width: 400 },
  beforeEach: seedRail({ open: true, width: 400 }),
};

/** Closed rail: no handle, the pane takes the whole window. */
export const Collapsed: Story = {
  args: { open: false },
  beforeEach: seedRail({ open: false, width: null }),
};

/** A narrow window: the pane keeps at least half the width, so the rail yields. */
export const NarrowWindow: Story = {
  args: { frameWidth: 440 },
  beforeEach: seedRail({ open: true, width: null }),
};

/**
 * The handle is a focusable separator: arrow keys step the width and persist
 * it, and a double-click returns to the default.
 */
export const KeyboardResize: Story = {
  tags: ["play-fn"],
  beforeEach: seedRail({ open: true, width: null }),
  play: async ({ canvasElement }) => {
    const handle = within(canvasElement).getByRole("separator", {
      name: "Resize sessions sidebar",
    });
    await waitFor(() =>
      expect(railWidth(canvasElement)).toBeCloseTo(SESSION_SIDEBAR_DEFAULT_WIDTH, 0)
    );

    handle.focus();
    await userEvent.keyboard("{ArrowRight}");
    await waitFor(() =>
      expect(railWidth(canvasElement)).toBeGreaterThan(SESSION_SIDEBAR_DEFAULT_WIDTH)
    );
    await waitFor(() =>
      expect(sessionSidebarStore.getSnapshot().context.width).toBe(
        Math.round(railWidth(canvasElement))
      )
    );

    await userEvent.dblClick(handle);
    await waitFor(() =>
      expect(railWidth(canvasElement)).toBeCloseTo(SESSION_SIDEBAR_DEFAULT_WIDTH, 0)
    );
    await expect(sessionSidebarStore.getSnapshot().context.width).toBeNull();
  },
};
