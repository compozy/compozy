import type { Meta, StoryObj } from "@storybook/react-vite";

import { UIProvider } from "../ui-provider";
import { StateGlyph, type StateGlyphState } from "../state-glyph";

const meta: Meta<typeof StateGlyph> = {
  title: "components/custom/StateGlyph",
  component: StateGlyph,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Work-state vocabulary: mint spinner ring (running), dashed ring (queued), filled mint check (done), Compozy orange dot (attention), subtle dot (idle). Every state shares one box so labels align row to row. Decorative by default; pass `label` when it stands alone.",
      },
    },
  },
  args: { state: "running" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const ROWS: ReadonlyArray<{ state: StateGlyphState; label: string }> = [
  { state: "running", label: "Running" },
  { state: "attention", label: "Needs you" },
  { state: "queued", label: "Queued" },
  { state: "done", label: "Done" },
  { state: "idle", label: "Idle" },
];

export const Default: Story = {};

/** The Tasks status column: glyph plus the plain state word. */
export const StatusColumn: Story = {
  render: () => (
    <div className="flex flex-col gap-4">
      {ROWS.map(row => (
        <span key={row.state} className="inline-flex items-center gap-2 text-body text-fg-2">
          <StateGlyph state={row.state} />
          {row.label}
        </span>
      ))}
    </div>
  ),
};

/** `sm` sits at the end of a dense transcript tool row. */
export const ToolRow: Story = {
  render: () => (
    <div className="flex w-96 items-center gap-2.5 rounded-lg bg-sunken px-4 py-2 text-body text-muted">
      <span className="font-medium text-fg">Run</span>
      <span className="min-w-0 truncate font-mono text-small-body">bun run test checkout</span>
      <span className="ml-auto inline-flex items-center gap-2 font-mono text-small-body text-subtle">
        <StateGlyph state="running" size="sm" />
        running
      </span>
    </div>
  ),
};

/** Under reduced motion the running ring holds still as a quarter arc. */
export const ReducedMotion: Story = {
  render: () => (
    <UIProvider reducedMotion="always">
      <span className="inline-flex items-center gap-2 text-body text-fg-2">
        <StateGlyph state="running" />
        Running
      </span>
    </UIProvider>
  ),
};

/** Standalone glyphs carry a `label` so they are announced. */
export const Labelled: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      {ROWS.map(row => (
        <StateGlyph key={row.state} state={row.state} label={row.label} />
      ))}
    </div>
  ),
};
