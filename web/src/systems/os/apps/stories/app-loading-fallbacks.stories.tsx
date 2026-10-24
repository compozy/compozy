import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { OsWindowSurface } from "../../components/os-window-frame";
import { OS_APPS } from "../../lib/app-registry";
import type { OsAppId } from "../../lib/os-types";

interface AppLoadingFallbackProps {
  app: OsAppId;
  pathname: string;
  search?: Record<string, unknown>;
  /** An unbound window on the All projects desktop. */
  aggregate?: boolean;
}

/**
 * One window frame (the real head, strip and body) whose body is the app's registered loading
 * fallback for a route: what a cold open shows while the app's code loads. No providers, no data.
 */
function AppLoadingFallback({
  app,
  pathname,
  search = {},
  aggregate = false,
}: AppLoadingFallbackProps) {
  const definition = OS_APPS[app];
  const Fallback = definition.LoadingFallback;
  const AppIcon = definition.icon;
  return (
    <div className="flex h-[600px] w-[1000px] overflow-hidden rounded-lg border border-line-strong bg-canvas">
      <OsWindowSurface controls="deck" glyph={<AppIcon />} title={definition.title}>
        {Fallback ? <Fallback aggregate={aggregate} route={{ pathname, search }} /> : null}
      </OsWindowSurface>
    </div>
  );
}

const meta = {
  title: "systems/os/components/AppLoadingFallbacks",
  component: AppLoadingFallback,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The body an OS window shows while its app's code loads, per app and route: static, query-free frames shaped like what the app renders, so the app arrives into the same places. Apps that publish a listing toolbar publish its placeholder into the window's strip row.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
} satisfies Meta<typeof AppLoadingFallback>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Session: Story = {
  args: { app: "session", pathname: "/agents/webgen/sessions/sess-1" },
};
export const SessionNoneSelected: Story = { args: { app: "session", pathname: "/sessions" } };

export const SettingsGeneral: Story = { args: { app: "settings", pathname: "/settings/general" } };
export const SettingsProviders: Story = {
  args: { app: "settings", pathname: "/settings/providers" },
};

export const TasksList: Story = { args: { app: "tasks", pathname: "/tasks" } };
export const TasksBoard: Story = {
  args: { app: "tasks", pathname: "/tasks", search: { mode: "kanban" } },
};
export const TasksInbox: Story = {
  args: { app: "tasks", pathname: "/tasks", search: { mode: "inbox" } },
};
export const TasksDashboard: Story = {
  args: { app: "tasks", pathname: "/tasks", search: { mode: "dashboard" } },
};
export const TaskDetail: Story = { args: { app: "tasks", pathname: "/tasks/task-1" } };
export const TaskRun: Story = { args: { app: "tasks", pathname: "/tasks/task-1/runs/run-1" } };

export const LoopsCatalog: Story = { args: { app: "loops", pathname: "/loops" } };
export const LoopsCatalogCards: Story = {
  args: { app: "loops", pathname: "/loops", search: { view: "cards" } },
};
export const LoopDetail: Story = { args: { app: "loops", pathname: "/loops/ship-it" } };
export const LoopRunForm: Story = { args: { app: "loops", pathname: "/loops/ship-it/run" } };
export const LoopEditor: Story = { args: { app: "loops", pathname: "/loops/ship-it/editor" } };
export const LoopConfigure: Story = {
  args: { app: "loops", pathname: "/loops/ship-it/configure" },
};
export const LoopRuns: Story = { args: { app: "loops", pathname: "/loop-runs" } };
export const LoopRunSteps: Story = {
  args: { app: "loops", pathname: "/loop-runs", search: { nodes: "waiting" } },
};
export const LoopRun: Story = { args: { app: "loops", pathname: "/loop-runs/run-1" } };
export const LoopRunComparison: Story = {
  args: { app: "loops", pathname: "/loop-runs/run-1/diff" },
};
export const LoopRunComparisonChosen: Story = {
  args: { app: "loops", pathname: "/loop-runs/run-1/diff", search: { against_run: "run-0" } },
};

export const AgentsFleet: Story = { args: { app: "agents", pathname: "/agents" } };
export const AgentOverview: Story = { args: { app: "agents", pathname: "/agents/webgen" } };
export const AgentInstructions: Story = {
  args: { app: "agents", pathname: "/agents/webgen", search: { tab: "instructions" } },
};
export const AgentInstructionsSoul: Story = {
  args: {
    app: "agents",
    pathname: "/agents/webgen",
    search: { tab: "instructions", file: "soul" },
  },
};
export const AgentConfiguration: Story = {
  args: { app: "agents", pathname: "/agents/webgen", search: { tab: "configuration" } },
};
export const AgentSessions: Story = {
  args: { app: "agents", pathname: "/agents/webgen", search: { tab: "sessions" } },
};

export const AutomationsListing: Story = { args: { app: "automations", pathname: "/automations" } };
export const AutomationJob: Story = {
  args: { app: "automations", pathname: "/automations/jobs/morning-digest" },
};
export const AutomationTrigger: Story = {
  args: { app: "automations", pathname: "/automations/triggers/rerun-delivery" },
};

export const MarketplaceBrowse: Story = { args: { app: "marketplace", pathname: "/marketplace" } };
export const MarketplaceInstalled: Story = {
  args: { app: "marketplace", pathname: "/marketplace/installed" },
};
export const MarketplaceEntry: Story = {
  args: { app: "marketplace", pathname: "/marketplace/compozy-catalog" },
};

export const Vault: Story = { args: { app: "vault", pathname: "/vault" } };

export const Dashboard: Story = { args: { app: "dashboard", pathname: "/" } };

export const NewTab: Story = { args: { app: "new-tab", pathname: "/new-tab" } };

export const Terminal: Story = { args: { app: "terminal", pathname: "/terminal" } };
export const TerminalById: Story = { args: { app: "terminal", pathname: "/terminal/term-1" } };
export const TerminalAcrossProjects: Story = {
  args: { app: "terminal", pathname: "/terminal", aggregate: true },
};

export const Usage: Story = { args: { app: "usage", pathname: "/usage" } };
