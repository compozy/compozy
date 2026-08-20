import type { Meta, StoryObj } from "@storybook/react-vite";

import { Kbd, KbdGroup } from "../kbd";

const meta: Meta<typeof Kbd> = {
  title: "components/ui/Kbd",
  component: Kbd,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Inline keyboard key indicator. Compose multiple keys with KbdGroup.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Single chord cap — the default read of every shortcut chip. */
export const Default: Story = {
  args: { children: "⌘K" },
};

/** One cap per key, composed with KbdGroup. */
export const Group: Story = {
  args: {},
  render: () => (
    <KbdGroup>
      <Kbd>⌘</Kbd>
      <Kbd>Shift</Kbd>
      <Kbd>P</Kbd>
    </KbdGroup>
  ),
};

/**
 * Glyph-run contract: modifiers, arrows, and letters render from one key
 * face with shared metrics — chord runs (⌘⇧P), editing keys (⏎ ⌫ ⎋),
 * and alternate chords separated by a muted slash between caps.
 */
export const GlyphRuns: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col items-start gap-3">
      <KbdGroup>
        <Kbd>⌘⇧P</Kbd>
        <Kbd>⌃K</Kbd>
        <Kbd>⌥⇥</Kbd>
      </KbdGroup>
      <KbdGroup>
        <Kbd>↑</Kbd>
        <Kbd>↓</Kbd>
        <Kbd>⏎</Kbd>
        <Kbd>⌫</Kbd>
        <Kbd>⎋</Kbd>
        <Kbd>esc</Kbd>
      </KbdGroup>
      <KbdGroup>
        <Kbd>⌘K</Kbd>
        <span aria-hidden="true" className="text-faint">
          /
        </span>
        <Kbd>⌘⇧P</Kbd>
      </KbdGroup>
    </div>
  ),
};
