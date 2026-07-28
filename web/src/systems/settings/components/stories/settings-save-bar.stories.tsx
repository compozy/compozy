import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { SettingsSaveBar } from "../settings-save-bar";

const meta: Meta<typeof SettingsSaveBar> = {
  title: "systems/settings/components/SettingsSaveBar",
  component: SettingsSaveBar,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Clean: Story = {
  args: {
    slug: "general",
    state: { kind: "idle" },
    onSave: fn(),
    onReset: fn(),
  },
};

export const Dirty: Story = {
  args: {
    slug: "general",
    state: { kind: "dirty", warnings: [] },
    onSave: fn(),
    onReset: fn(),
  },
};

export const Saving: Story = {
  args: {
    slug: "general",
    state: { kind: "saving" },
    onSave: fn(),
    onReset: fn(),
  },
};

export const Invalid: Story = {
  args: {
    slug: "general",
    state: { kind: "invalid", warnings: [] },
    onSave: fn(),
    onReset: fn(),
  },
};

export const WithError: Story = {
  args: {
    slug: "general",
    state: { kind: "error", message: "Could not reach the daemon" },
    onSave: fn(),
    onReset: fn(),
  },
};

export const WithWarnings: Story = {
  args: {
    slug: "general",
    state: {
      kind: "warning",
      message: "Saved with warnings",
      warnings: ["Restart required", "Sandbox mismatch"],
    },
    onSave: fn(),
    onReset: fn(),
  },
};

export const LastApplied: Story = {
  args: {
    slug: "general",
    state: { kind: "saved", message: "Applied 2 minutes ago" },
    onSave: fn(),
    onReset: fn(),
  },
};

/**
 * Interaction test: flips isDirty true → false via Discard and asserts the
 * save bar leaves the page once the draft is clean.
 */
export const DirtyToCleanInteraction: Story = {
  tags: ["play-fn"],
  render: () => {
    function Harness() {
      const [isDirty, setIsDirty] = useState(true);
      return (
        <SettingsSaveBar
          slug="general"
          state={isDirty ? { kind: "dirty", warnings: [] } : { kind: "idle" }}
          onSave={() => setIsDirty(false)}
          onReset={() => setIsDirty(false)}
        />
      );
    }
    return <Harness />;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const reset = canvas.getByTestId("settings-page-general-reset");
    await expect(reset).not.toBeDisabled();
    await userEvent.click(reset);
    await expect(canvas.queryByTestId("settings-page-general-save-bar")).not.toBeInTheDocument();
  },
};
