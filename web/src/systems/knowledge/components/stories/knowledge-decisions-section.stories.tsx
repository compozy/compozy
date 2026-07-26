import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import type { MemoryDecision } from "@/systems/knowledge/types";

import { KnowledgeDecisionsSection } from "@/systems/knowledge/components/knowledge-decisions-section";

const meta: Meta<typeof KnowledgeDecisionsSection> = {
  title: "systems/knowledge/components/KnowledgeDecisionsSection",
  component: KnowledgeDecisionsSection,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const baseDecision: MemoryDecision = {
  id: "dec_alpha",
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

const llmDecision: MemoryDecision = {
  ...baseDecision,
  id: "dec_beta",
  op: "add",
  source: "llm",
  applied_at: "2026-04-17T17:32:00Z",
  reason: "llm:multi-slot-tiebreaker",
  llm_trace: { latency_ms: 240, model: "haiku", prompt_version: "controller@v1" },
};

const pendingDecision: MemoryDecision = {
  ...baseDecision,
  id: "dec_gamma",
  op: "delete",
  applied_at: null,
  reason: "rule:explicit-delete",
};

export const Default: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDecisionsSection
        decisions={[baseDecision, llmDecision, pendingDecision]}
        error={null}
        isLoading={false}
      />
    </PanelSurface>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDecisionsSection decisions={[]} error={null} isLoading />
    </PanelSurface>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDecisionsSection
        decisions={[]}
        error={new globalThis.Error("Decisions failed")}
        isLoading={false}
      />
    </PanelSurface>
  ),
};

export const Empty: Story = {
  args: {},
  render: () => (
    <PanelSurface>
      <KnowledgeDecisionsSection decisions={[]} error={null} isLoading={false} />
    </PanelSurface>
  ),
};
