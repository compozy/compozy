import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { sessionSidebarStore } from "@/systems/session/hooks/use-session-sidebar-state";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

import { SessionWindowSkeleton } from "../session-window-skeleton";

/** Stories render outside an OS window, so the rail reads the shell scope. */
function seedRail(open: boolean) {
  return () => {
    sessionSidebarStore.trigger.sidebarVisibilityChanged({ scopeId: SHELL_WORKTREE_SCOPE, open });
    return () => {
      sessionSidebarStore.trigger.sidebarVisibilityChanged({
        scopeId: SHELL_WORKTREE_SCOPE,
        open: true,
      });
    };
  };
}

const meta = {
  title: "systems/os/components/SessionWindowSkeleton",
  component: SessionWindowSkeleton,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A session window whose session has not loaded: the rail-and-pane split the loaded window mounts, with the rail's shell over placeholder rows and the pane as a transcript over a composer card. Every frame the loaded window fills is already in place, so the swap moves nothing. One busy status for the whole window; no queries, so hidden windows load cheaply.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex h-[560px] w-[960px] overflow-hidden rounded-lg border border-line-strong bg-canvas">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
} satisfies Meta<typeof SessionWindowSkeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The rail open at its saved width, beside the transcript and composer placeholders. */
export const Default: Story = {
  beforeEach: seedRail(true),
};

/** A closed rail: the pane takes the whole window, as the loaded window will. */
export const RailClosed: Story = {
  beforeEach: seedRail(false),
};
