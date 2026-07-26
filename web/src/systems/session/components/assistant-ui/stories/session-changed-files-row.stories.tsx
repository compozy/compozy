import type { Meta, StoryObj } from "@storybook/react-vite";

import { SessionChangedFilesRowView } from "@/components/assistant-ui/session-changed-files-row";
import type { SessionChangedFilesRow } from "@/components/assistant-ui/session-timeline.logic";

function row(expanded: boolean): SessionChangedFilesRow {
  return {
    kind: "changed-files",
    id: "changed-files:story-turn",
    turnId: "story-turn",
    expanded,
    additions: 9,
    deletions: 3,
    files: [
      { path: "/workspace/app/src/launch-note.ts", additions: 1, deletions: 1 },
      { path: "/workspace/app/src/config/flags.ts", additions: 3, deletions: 2 },
      { path: "/workspace/RELEASE.md", additions: 5, deletions: 0 },
    ],
  };
}

/**
 * The settled-turn changed-files roll-up card in isolation. Collapsed it reads
 * `Edited N files +a/-d`; expanded it lists each modified file with diff stats.
 * Styling per `analysis/07 §4.2 #26`: `--radius-lg` + `border-line` +
 * `bg-canvas-soft` + `<Eyebrow>` header — flat depth, no `bg-card/45` translucency.
 * Display-only (no Undo/Review): AGH exposes no checkpoint semantics.
 */
const meta: Meta<typeof SessionChangedFilesRowView> = {
  title: "systems/session/components/assistant-ui/ChangedFilesRollup",
  component: SessionChangedFilesRowView,
  parameters: { layout: "padded" },
  decorators: [
    Story => (
      <div className="max-w-[560px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Collapsed: Story = {
  args: { row: row(false), onToggle: () => undefined },
};

export const Expanded: Story = {
  args: { row: row(true), onToggle: () => undefined },
};
