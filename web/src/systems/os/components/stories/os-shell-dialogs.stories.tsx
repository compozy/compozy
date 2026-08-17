import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  STORY_SHORTCUT_DEFAULTS,
  STORY_SHORTCUT_REBOUND_OVERRIDES,
} from "@/storybook/window-manager-shortcut-fixtures";

import { OsShellContext } from "../../contexts/os-shell-context";
import { effectiveShortcutMap } from "../../lib/window-manager-shortcuts";
import type { WindowManagerConfig } from "../../lib/window-manager-types";
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
          "The two shell-scoped dialogs the Help and CompozyOS menus open: the keyboard reference (every registry action, bound or not) and the installation identity read straight from `/api/status`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function shortcutConfig(overrides: WindowManagerConfig["shortcuts"]): WindowManagerConfig {
  const base = createLiveStoryShell().manager.getState().windowManagerConfig;
  if (base === null) throw new Error("Story shell requires window-manager config");
  return {
    ...base,
    shortcuts: overrides,
    shortcutDefaults: STORY_SHORTCUT_DEFAULTS,
    effectiveShortcuts: effectiveShortcutMap(STORY_SHORTCUT_DEFAULTS, overrides),
  };
}

function DialogFixture({
  dialog,
  windowManagerConfig,
}: {
  dialog: "shortcuts" | "about";
  windowManagerConfig?: WindowManagerConfig;
}) {
  const [shell] = useState(() => createLiveStoryShell({ windowManagerConfig }));
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
  render: () => <DialogFixture dialog="shortcuts" windowManagerConfig={shortcutConfig({})} />,
};

/** Rebound actions are marked from the daemon's effective map. */
export const ShortcutsRebound: Story = {
  args: { open: true, onOpenChange: fn() },
  render: () => (
    <DialogFixture
      dialog="shortcuts"
      windowManagerConfig={shortcutConfig(STORY_SHORTCUT_REBOUND_OVERRIDES)}
    />
  ),
};

/** Installation identity: only fields the daemon actually publishes. */
export const About: Story = {
  args: { open: true, onOpenChange: fn() },
  render: () => <DialogFixture dialog="about" />,
};
