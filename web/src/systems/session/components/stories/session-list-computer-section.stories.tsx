import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  LOCAL_COMPUTER_FIXTURE,
  makeComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import { sessionLifecycleActionsFixture } from "../../mocks/session-row-fixtures";
import {
  SessionListOfflineNotice,
  SessionListScopeRemovedNotice,
} from "../session-list/session-list-scope-notice";
import { SessionListComputerSection } from "../session-list/session-list-computer-section";
import { SessionListTotals } from "../session-list/session-list-totals";

const EMPTY_SET = new Set<string>();
const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function sourceFixture(
  overrides: Partial<ComputerSessionsSource> & { key: string; name: string }
): ComputerSessionsSource {
  return {
    isLocal: overrides.key === "local",
    computer: LOCAL_COMPUTER_FIXTURE,
    read: "ok",
    detail: overrides.key === "local" ? "This computer" : "via LAN",
    canOperate: true,
    canCreate: true,
    facets: undefined,
    count: 0,
    needsYou: 0,
    working: 0,
    laterCount: 0,
    workspaces: [],
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: null,
    retry: fn(),
    ...overrides,
  };
}

const meta: Meta<typeof SessionListComputerSection> = {
  title: "systems/session/components/SessionListComputerSection",
  component: SessionListComputerSection,
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    Story => (
      <QueryClientProvider client={client}>
        <CenteredSurface>
          <div className="flex w-72 flex-col gap-2 rounded-lg border border-line-strong bg-sunken p-2.5">
            <Story />
          </div>
        </CenteredSurface>
      </QueryClientProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const ComputerStates: Story = {
  render: () => {
    const sources: ComputerSessionsSource[] = [
      sourceFixture({
        key: "cmp_connecting",
        name: "hetzner",
        read: "connecting",
        detail: "Connecting…",
        count: null,
        computer: makeComputerFixture({
          id: "cmp_connecting",
          name: "hetzner",
          status: "connecting",
        }),
      }),
      sourceFixture({
        key: "cmp_failed",
        name: "devbox",
        read: "failed",
        detail: "via LAN",
        count: null,
        computer: makeComputerFixture({ id: "cmp_failed", name: "devbox", status: "connected" }),
      }),
      sourceFixture({
        key: "cmp_offline",
        name: "workstation",
        read: "offline_empty",
        detail: "Offline · 2h ago",
        count: null,
        canOperate: false,
        canCreate: false,
        computer: makeComputerFixture({
          id: "cmp_offline",
          name: "workstation",
          status: "offline",
        }),
      }),
      sourceFixture({
        key: "cmp_pro",
        name: "mini-build",
        read: "needs_pro",
        detail: "Needs Pro",
        count: null,
        canOperate: false,
        canCreate: false,
        computer: makeComputerFixture({ id: "cmp_pro", name: "mini-build", status: "needs_pro" }),
      }),
      sourceFixture({
        key: "cmp_old",
        name: "gpu-vm",
        read: "incompatible",
        detail: "Update needed",
        count: null,
        canOperate: false,
        canCreate: false,
        computer: makeComputerFixture({ id: "cmp_old", name: "gpu-vm", status: "incompatible" }),
      }),
    ];
    return (
      <>
        {sources.map(source => (
          <SessionListComputerSection
            collapsedThreads={EMPTY_SET}
            groupBy="project"
            key={source.key}
            onNewSession={fn()}
            onOpenComputerDetails={fn()}
            onSelectSession={fn()}
            onThreadBranchChange={fn()}
            search=""
            sessionActions={sessionLifecycleActionsFixture()}
            source={source}
            subagentThreads={{ has: () => false }}
            testIdPrefix="story"
          />
        ))}
        <SessionListTotals
          testIdPrefix="story"
          totals={{ needsYou: 1, working: 2, idle: 4 }}
          uncountedNames={["hetzner", "devbox", "workstation", "gpu-vm"]}
        />
      </>
    );
  },
};

export const ScopeNotices: Story = {
  render: () => (
    <>
      <SessionListOfflineNotice
        computer={makeComputerFixture({
          id: "cmp_ws",
          name: "workstation",
          status: "offline",
          lastConnectedLabel: "2h ago",
        })}
        onReconnect={fn()}
      />
      <SessionListScopeRemovedNotice name="devbox" onDismiss={fn()} />
    </>
  ),
};
