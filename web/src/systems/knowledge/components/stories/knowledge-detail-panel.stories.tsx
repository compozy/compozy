import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyAgentNames, storyWorkspaceIds } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";
import type { MemoryDecision, MemoryHeader } from "@/systems/knowledge/types";

import { KnowledgeDetailPanel } from "@/systems/knowledge/components/knowledge-detail-panel";

const meta: Meta<typeof KnowledgeDetailPanel> = {
  title: "systems/knowledge/components/KnowledgeDetailPanel",
  component: KnowledgeDetailPanel,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const defaultMemory: MemoryHeader = {
  filename: "operator-style.md",
  mod_time: "2026-04-17T17:30:00Z",
  name: "Operator Style",
  scope: "global",
  type: "user",
  recall_count: 4,
  last_recalled_at: "2026-04-17T17:25:00Z",
  injection: true,
  system_managed: false,
  description: "Northstar guidance for concise, accountable operator communication.",
};

const agentMemory: MemoryHeader = {
  filename: "cto-tone.md",
  mod_time: "2026-04-17T17:25:00Z",
  name: "CTO Tone",
  scope: "agent",
  type: "user",
  recall_count: 6,
  last_recalled_at: "2026-04-17T17:20:00Z",
  injection: true,
  system_managed: false,
  description: "Direct, calm tone for CTO summaries; lead with the next decision.",
  agent_name: storyAgentNames.cto,
  agent_tier: "workspace",
  workspace_id: storyWorkspaceIds.hq,
};

const supersededMemory: MemoryHeader = {
  ...defaultMemory,
  filename: "operator-style-v1.md",
  staleness_banner: "Updated >7 days after last recall",
  superseded_by: "operator-style-v2.md",
};

const defaultContent = [
  "# Operator Style",
  "",
  "Own the launch outcome end to end.",
  "",
  "- Lead with the current fact pattern and the next owner.",
  "- Escalate ambiguity with the next concrete evidence request.",
  "- Keep customer-facing language calm, specific, and launch-safe.",
].join("\n");

const sampleDecision: MemoryDecision = {
  id: "dec_demo",
  candidate_hash: "h",
  op: "update",
  scope: "global",
  source: "rule",
  confidence: 0.93,
  decided_at: "2026-04-17T17:31:00Z",
  applied_at: "2026-04-17T17:31:01Z",
  target_filename: "operator-style.md",
  reason: "rule:exact-slug-collision",
  frontmatter: {
    filename: "operator-style.md",
    mod_time: "2026-04-17T17:30:00Z",
    name: "Operator Style",
    type: "user",
  },
};

export const Default: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={defaultContent}
        decisions={[sampleDecision]}
        decisionsError={null}
        deleteError={null}
        editError={null}
        error={null}
        status={{
          isDecisionsLoading: false,
          isDeletePending: false,
          isEditPending: false,
          isLoading: false,
        }}
        memory={defaultMemory}
        onDelete={async () => {}}
        onEdit={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};

export const AgentScope: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={defaultContent}
        decisions={[]}
        decisionsError={null}
        deleteError={null}
        editError={null}
        error={null}
        status={{
          isDecisionsLoading: false,
          isDeletePending: false,
          isEditPending: false,
          isLoading: false,
        }}
        memory={agentMemory}
        onDelete={async () => {}}
        onEdit={async () => {}}
        scope="agent"
      />
    </PanelSurface>
  ),
};

export const Superseded: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={defaultContent}
        decisions={[sampleDecision]}
        decisionsError={null}
        deleteError={null}
        editError={null}
        error={null}
        status={{
          isDecisionsLoading: false,
          isDeletePending: false,
          isEditPending: false,
          isLoading: false,
        }}
        memory={supersededMemory}
        onDelete={async () => {}}
        onEdit={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={undefined}
        decisions={[]}
        error={null}
        status={{ isDeletePending: false, isLoading: true }}
        memory={defaultMemory}
        onDelete={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={undefined}
        decisions={[]}
        error={new globalThis.Error("Content fetch failed")}
        status={{ isDeletePending: false, isLoading: false }}
        memory={defaultMemory}
        onDelete={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};

export const EmptySelection: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={undefined}
        decisions={[]}
        error={null}
        status={{ isDeletePending: false, isLoading: false }}
        memory={undefined}
        onDelete={async () => {}}
      />
    </PanelSurface>
  ),
};

export const DecisionsLoading: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={defaultContent}
        decisions={[]}
        decisionsError={null}
        deleteError={null}
        error={null}
        status={{ isDecisionsLoading: true, isDeletePending: false, isLoading: false }}
        memory={defaultMemory}
        onDelete={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};

export const DecisionsError: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDetailPanel
        content={defaultContent}
        decisions={[]}
        decisionsError={new globalThis.Error("Decisions failed")}
        deleteError={null}
        error={null}
        status={{ isDecisionsLoading: false, isDeletePending: false, isLoading: false }}
        memory={defaultMemory}
        onDelete={async () => {}}
        scope="global"
      />
    </PanelSurface>
  ),
};
