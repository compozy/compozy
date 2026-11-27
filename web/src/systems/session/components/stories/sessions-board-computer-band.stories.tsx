import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  LOCAL_COMPUTER,
  makeComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionsBoardComputerBand } from "../../hooks/use-sessions-board-computers";
import { SessionsBoardComputerBandView } from "../session-board/sessions-board-computer-band";
import type { SessionsBoardRow } from "../session-board/sessions-board-view";

const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

const BASE_ROW: SessionsBoardRow = {
  currentSessionId: null,
  collapsedColumns: {},
  onColumnCollapsedChange: fn(),
  onOpen: fn(),
  workspaceNameOf: () => null,
};

function bandFixture(
  overrides: Partial<Omit<SessionsBoardComputerBand, "source">> & {
    source: Partial<ComputerSessionsSource> & { key: string; name: string };
  }
): SessionsBoardComputerBand {
  const { source, ...rest } = overrides;
  return {
    collapsed: false,
    onToggle: fn(),
    baseFilters: {},
    lanes: null,
    boardCounts: { needs_you: 2, working: 1, waiting: 1, in_review: 1, done: 3 },
    workspaceNameOf: () => null,
    ...rest,
    source: {
      isLocal: source.key === "local",
      computer: LOCAL_COMPUTER,
      read: "ok",
      detail: source.key === "local" ? "This computer" : "via LAN",
      canOperate: true,
      canCreate: true,
      facets: undefined,
      count: 8,
      needsYou: 2,
      working: 1,
      laterCount: 0,
      workspaces: [],
      workspacesLoading: false,
      workspacesError: false,
      matchedWorkspaceIds: null,
      retry: fn(),
      ...source,
    },
  };
}

const meta: Meta<typeof SessionsBoardComputerBandView> = {
  title: "systems/session/components/SessionsBoardComputerBand",
  component: SessionsBoardComputerBandView,
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    Story => (
      <QueryClientProvider client={client}>
        <CenteredSurface>
          <div className="flex w-full max-w-4xl flex-col gap-4 rounded-lg border border-line-strong bg-canvas p-4">
            <Story />
          </div>
        </CenteredSurface>
      </QueryClientProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const FoldedBandCounts: Story = {
  args: {
    band: bandFixture({
      collapsed: true,
      source: {
        key: "cmp_devbox",
        name: "devbox",
        computer: makeComputerFixture({ id: "cmp_devbox", name: "devbox", status: "connected" }),
      },
    }),
    rank: 0,
    row: BASE_ROW,
  },
};

export const FailedBand: Story = {
  args: {
    band: bandFixture({
      collapsed: false,
      boardCounts: undefined,
      source: {
        key: "cmp_fail",
        name: "hetzner",
        read: "failed",
        count: null,
        needsYou: 0,
        computer: makeComputerFixture({ id: "cmp_fail", name: "hetzner", status: "connected" }),
      },
    }),
    rank: 1,
    row: BASE_ROW,
  },
};
