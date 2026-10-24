import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { TerminalJournalPanel } from "../terminal-journal-panel";
import {
  TerminalAggregateSkeleton,
  TerminalPipeLogSkeleton,
  TerminalRecordingSkeleton,
  TerminalWindowSkeleton,
} from "../terminal-skeletons";
import { TerminalVisualStage } from "./terminal-visual-stage";

/**
 * The Terminal app's surfaces while their data is read. Each stands in the frame the loaded
 * surface fills, so the swap moves nothing. The emulator itself is never a placeholder: the live
 * grid keeps zero animation and these hold its place until it mounts.
 */
const meta: Meta = {
  title: "systems/terminal/components/TerminalSkeletons",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The window before its terminal is known: the emulator's dark ground with a few mono lines. */
export const WindowBody: Story = {
  render: () => (
    <TerminalVisualStage width="default">
      <TerminalWindowSkeleton label="Loading terminals" />
    </TerminalVisualStage>
  ),
};

/** The all-projects list: the filter and the new-terminal row as blocks, over rows of `Item`. */
export const AggregateList: Story = {
  render: () => (
    <TerminalVisualStage width="default">
      <TerminalAggregateSkeleton />
    </TerminalVisualStage>
  ),
};

/** A journal read in flight: the toolbar stays live, the table keeps its real column heads. */
export const JournalLoading: Story = {
  render: () => (
    <TerminalVisualStage>
      <TerminalJournalPanel
        chips={[]}
        entries={[]}
        hasMore={false}
        loading
        onFiltersChange={fn()}
        onLoadMore={fn()}
      />
    </TerminalVisualStage>
  ),
};

/** A pipe terminal's captured output: the numbered gutter of the real log pane. */
export const PipeLog: Story = {
  render: () => (
    <TerminalVisualStage width="default">
      <TerminalPipeLogSkeleton />
    </TerminalVisualStage>
  ),
};

/** A recording's artifact being read: title row, terminal ground and transport bar. */
export const Recording: Story = {
  render: () => (
    <TerminalVisualStage width="default">
      <TerminalRecordingSkeleton />
    </TerminalVisualStage>
  ),
};
