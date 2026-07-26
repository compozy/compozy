import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { storyAgentNames, storyWorkspaceIds } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import { knowledgeMemoryKey } from "@/systems/knowledge";
import type { KnowledgeMemoryItem } from "@/systems/knowledge/types";

import { KnowledgeListPanel } from "@/systems/knowledge/components/knowledge-list-panel";

const meta: Meta<typeof KnowledgeListPanel> = {
  title: "systems/knowledge/components/KnowledgeListPanel",
  component: KnowledgeListPanel,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const defaultMemories: KnowledgeMemoryItem[] = [
  {
    filename: "operator-style.md",
    key: "global:operator-style.md",
    mod_time: "2026-04-17T17:30:00Z",
    name: "Operator Style",
    scope: "global",
    type: "user",
    recall_count: 0,
    injection: true,
    system_managed: false,
    description: "Northstar guidance for concise, accountable operator communication.",
  },
  {
    filename: "launch-week-brief.md",
    key: "global:launch-week-brief.md",
    mod_time: "2026-04-17T09:00:00Z",
    name: "Launch Week Brief",
    scope: "global",
    type: "project",
    recall_count: 4,
    injection: true,
    system_managed: false,
    description: "Shared context for launch KPIs, cutover timing, and cross-functional owners.",
  },
  {
    filename: "executive-risk-memo.md",
    key: "workspace:executive-risk-memo.md",
    mod_time: "2026-04-17T16:10:00Z",
    name: "Executive Risk Memo",
    scope: "workspace",
    type: "reference",
    recall_count: 1,
    injection: true,
    system_managed: false,
    description:
      "Workspace-local memo with launch blockers, fallback paths, and decision thresholds.",
    agent_name: storyAgentNames.cto,
    workspace_id: storyWorkspaceIds.hq,
    staleness_banner: "Updated >7 days after last recall",
  },
  {
    filename: "support-macro-pack.md",
    key: "workspace:support-macro-pack.md",
    mod_time: "2026-04-17T14:45:00Z",
    name: "Support Macro Pack",
    scope: "workspace",
    type: "reference",
    recall_count: 0,
    injection: true,
    system_managed: false,
    description:
      "Approved language for pricing questions, launch delays, and high-touch merchant callbacks.",
    workspace_id: storyWorkspaceIds.hq,
  },
  {
    filename: "cto-tone.md",
    key: "agent:cto-tone.md",
    mod_time: "2026-04-17T17:25:00Z",
    name: "CTO Tone",
    scope: "agent",
    type: "user",
    recall_count: 6,
    injection: true,
    system_managed: false,
    description: "Direct, calm tone for CTO summaries; lead with the next decision.",
    agent_name: storyAgentNames.cto,
    agent_tier: "workspace",
    workspace_id: storyWorkspaceIds.hq,
  },
];

export const Default: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={defaultMemories}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={defaultMemories[0] ? knowledgeMemoryKey(defaultMemories[0]) : null}
      />
    </PanelSurface>
  ),
};

export const Empty: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={[]}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const SearchActive: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={defaultMemories.slice(0, 1)}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchInfo="Recall 1 of top-K"
        searchMode
        searchQuery="operator"
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const SearchEmpty: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={[]}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchInfo="Recall 0 of top-K"
        searchMode
        searchQuery="zzzzzz"
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        isLoading
        memories={[]}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        errorMessage="Network failure while loading memories"
        memories={[]}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const AgentScopeOnly: Story = {
  args: {},
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={defaultMemories.filter(memory => memory.scope === "agent")}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={null}
      />
    </PanelSurface>
  ),
};

export const RowSelect: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => (
    <PanelSurface className="max-w-[360px]">
      <KnowledgeListPanel
        memories={defaultMemories}
        onSearchChange={() => undefined}
        onSelectMemory={() => undefined}
        searchQuery=""
        selectedMemoryKey={defaultMemories[0] ? knowledgeMemoryKey(defaultMemories[0]) : null}
      />
    </PanelSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const row = await canvas.findByTestId(`memory-item-${knowledgeMemoryKey(defaultMemories[2])}`);
    await userEvent.click(row);
    await expect(row).toBeVisible();
  },
};
