import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { OsShellContext } from "../../contexts/os-shell-context";
import type {
  LayoutTab,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../../lib/window-manager-types";
import { OsTabBar } from "../os-tab-bar";
import { createLiveStoryShell } from "./_shell-fixture";

const meta: Meta = {
  title: "systems/os/components/OsTabBar",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The active desktop's one bar, between the menubar and the desk. Pinned tabs lead behind a hairline, regular tabs shrink to fit, then the New tab button; once tabs reach the 40px floor the strip scrolls and the overflow list names every tab. Right-click a tab for Pin, Move to desktop, and the bulk closes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const DESKTOP_ID = "desktop:bar";

function storyWindow(id: string, app: string, tabId: string): WindowManagerWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: DESKTOP_ID,
    tabId,
  };
}

function solo(windowId: string, pinned = false): LayoutTab {
  return {
    id: `tab:${windowId}`,
    pinned,
    root: { id: `leaf:${windowId}`, kind: "leaf", windowId },
  };
}

/** Pinned Dashboard, then single tabs and a two-window split; `extra` single tabs crowd the bar. */
function barSnapshot(extra = 0): WindowManagerSnapshot {
  const apps = ["tasks", "loops", "files", "vault", "usage", "automations", "marketplace"];
  const extraIds = Array.from({ length: extra }, (_, index) => `w-extra-${index}`);
  const tabs: LayoutTab[] = [
    solo("w-dash", true),
    solo("w-tasks"),
    {
      id: "tab:split",
      pinned: false,
      root: {
        id: "split:agents-terminal",
        kind: "split",
        axis: "horizontal",
        weights: [0.5, 0.5],
        children: [
          { id: "leaf:w-agents", kind: "leaf", windowId: "w-agents" },
          { id: "leaf:w-terminal", kind: "leaf", windowId: "w-terminal" },
        ],
      },
    },
    solo("w-files"),
    ...extraIds.map(id => solo(id)),
  ];
  const windows: Record<string, WindowManagerWindow> = {
    "w-dash": storyWindow("w-dash", "dashboard", "tab:w-dash"),
    "w-tasks": storyWindow("w-tasks", "tasks", "tab:w-tasks"),
    "w-agents": storyWindow("w-agents", "agents", "tab:split"),
    "w-terminal": storyWindow("w-terminal", "terminal", "tab:split"),
    "w-files": storyWindow("w-files", "files", "tab:w-files"),
  };
  extraIds.forEach((id, index) => {
    windows[id] = storyWindow(id, apps[index % apps.length]!, `tab:${id}`);
  });
  return {
    version: 6,
    workspaceId: "workspace-compozy",
    revision: 4,
    desktops: [{ id: DESKTOP_ID, name: "Launch", order: 0, tabs, activeTabId: "tab:w-tasks" }],
    windows,
    closedEntryCount: 1,
    overrides: {},
    migration: null,
    updatedAt: "2026-11-12T00:00:00Z",
  };
}

function LiveBar({ extra, width }: { extra?: number; width?: number }) {
  const [shell] = useState(() =>
    createLiveStoryShell({ snapshot: barSnapshot(extra), focusedWindowId: "w-tasks" })
  );
  return (
    <OsShellContext.Provider value={shell}>
      <div className="bg-rail" style={{ width: width ?? "100%" }}>
        <OsTabBar />
      </div>
    </OsShellContext.Provider>
  );
}

/** Room for every tab at its widest: pinned Dashboard, Tasks active, a split, Files. */
export const Resting: Story = { render: () => <LiveBar /> };

/** More tabs than fit at 224px: regular tabs shrink, shedding detail from the end. */
export const Shrinking: Story = { render: () => <LiveBar extra={6} width={960} /> };

/** Tabs at the floor still overflow: the strip scrolls and the list names all of them. */
export const Overflow: Story = { render: () => <LiveBar extra={14} width={640} /> };
