import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { userEvent, within } from "storybook/test";

import type { ShortcutMap } from "@/systems/os";
import { CenteredSurface } from "@/storybook/story-layout";
import {
  STORY_SHORTCUT_DEFAULTS,
  STORY_SHORTCUT_DIAGNOSTIC_OVERRIDES,
} from "@/storybook/window-manager-shortcut-fixtures";

import { useWindowManagerShortcutRecorder } from "../../hooks/use-window-manager-shortcut-recorder";
import { ShortcutPresetCard } from "../layouts/shortcut-preset-card";
import { WindowManagerShortcutTable } from "../layouts/window-manager-shortcut-table";

function ShortcutTableFixture({ initial }: { initial: ShortcutMap }) {
  const [overrides, setOverrides] = useState(initial);
  const recorder = useWindowManagerShortcutRecorder(
    overrides,
    STORY_SHORTCUT_DEFAULTS,
    setOverrides
  );
  return (
    <div className="max-h-190 w-240 overflow-y-auto rounded-lg border border-line bg-canvas-soft">
      <WindowManagerShortcutTable
        defaults={STORY_SHORTCUT_DEFAULTS}
        overrides={overrides}
        recorder={recorder}
      />
    </div>
  );
}

function PresetFixture({ initial }: { initial: ShortcutMap }) {
  const [overrides, setOverrides] = useState(initial);
  return (
    <div className="w-240 overflow-hidden rounded-lg border border-line bg-canvas-soft">
      <ShortcutPresetCard
        defaults={STORY_SHORTCUT_DEFAULTS}
        overrides={overrides}
        onChange={setOverrides}
      />
    </div>
  );
}

const meta: Meta<typeof ShortcutTableFixture> = {
  title: "systems/settings/components/WindowManagerShortcutContract",
  component: ShortcutTableFixture,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const TableDefaults: Story = {
  args: { initial: {} },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /^Shell/ }));
    await userEvent.click(canvas.getByRole("button", { name: /^Window/ }));
  },
};

export const BlockedAndPartialRange: Story = {
  args: { initial: STORY_SHORTCUT_DIAGNOSTIC_OVERRIDES },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /^Shell/ }));
    await userEvent.click(canvas.getByRole("button", { name: /^Window/ }));
  },
};

export const ShadowedSurface: Story = {
  args: { initial: { "sidebar.toggle": ["meta+KeyB"] } },
};

export const PresetPreview: Story = {
  args: { initial: {} },
  render: () => <PresetFixture initial={{}} />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Preview" }));
  },
};

export const PresetApplied: Story = {
  args: { initial: {} },
  render: () => <PresetFixture initial={{}} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Preview" }));
    await userEvent.click(canvas.getByRole("button", { name: "Apply preset" }));
  },
};
