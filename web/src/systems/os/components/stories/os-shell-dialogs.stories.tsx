import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { OsShellContext } from "../../contexts/os-shell-context";
import { OsAboutDialog } from "../os-about-dialog";
import { OsShortcutsDialog } from "../os-shortcuts-dialog";
import { createLiveStoryShell } from "./_shell-fixture";
import { DesktopShell } from "./_desktop";

const meta: Meta<typeof OsShortcutsDialog> = {
  title: "systems/os/components/OsShellDialogs",
  component: OsShortcutsDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The two shell-scoped dialogs the Help and AGH menus open: the keyboard reference (every registry action, bound or not) and the installation identity read straight from `/api/status`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function DialogFixture({ dialog }: { dialog: "shortcuts" | "about" }) {
  const [shell] = useState(createLiveStoryShell);
  return (
    <OsShellContext.Provider value={shell}>
      <DesktopShell wallpaper="carbon">
        {dialog === "shortcuts" ? (
          <OsShortcutsDialog open onOpenChange={fn()} />
        ) : (
          <OsAboutDialog open onOpenChange={fn()} />
        )}
      </DesktopShell>
    </OsShellContext.Provider>
  );
}

/** Shell, Window, Layout, and Desktops sections over the live registry. */
export const Shortcuts: Story = {
  args: { open: true, onOpenChange: fn() },
  render: () => <DialogFixture dialog="shortcuts" />,
};

/** Installation identity: only fields the daemon actually publishes. */
export const About: Story = {
  args: { open: true, onOpenChange: fn() },
  render: () => <DialogFixture dialog="about" />,
};
