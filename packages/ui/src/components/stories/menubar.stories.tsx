import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  Menubar,
  MenubarCheckboxItem,
  MenubarContent,
  MenubarGroup,
  MenubarItem,
  MenubarLabel,
  MenubarMenu,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "../menubar";

const meta: Meta<typeof Menubar> = {
  title: "components/ui/Menubar",
  component: Menubar,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Persistent application menu bar. Arrow keys traverse the triggers, Home/End jump to the ends, and once one menu is open, hovering a sibling switches to it. Popups share the DropdownMenu surface, so both menu systems read as one.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Menubar>
      <MenubarMenu>
        <MenubarTrigger>Session</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>
            New session
            <MenubarShortcut>⌘N</MenubarShortcut>
          </MenubarItem>
          <MenubarItem>New agent…</MenubarItem>
          <MenubarSeparator />
          <MenubarItem>Sessions…</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Window</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>
            Minimize
            <MenubarShortcut>⌘M</MenubarShortcut>
          </MenubarItem>
          <MenubarItem disabled>Zoom</MenubarItem>
          <MenubarSeparator />
          <MenubarSub>
            <MenubarSubTrigger>Move &amp; resize</MenubarSubTrigger>
            <MenubarSubContent>
              <MenubarItem>
                Left half
                <MenubarShortcut>⌃⌥←</MenubarShortcut>
              </MenubarItem>
              <MenubarItem>
                Right half
                <MenubarShortcut>⌃⌥→</MenubarShortcut>
              </MenubarItem>
            </MenubarSubContent>
          </MenubarSub>
          <MenubarSeparator />
          <MenubarItem variant="destructive">
            Close window
            <MenubarShortcut>⌘W</MenubarShortcut>
          </MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Help</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>Keyboard shortcuts…</MenubarItem>
          <MenubarSeparator />
          <MenubarItem>Documentation</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  ),
};

function StatefulMenubar() {
  const [density, setDensity] = useState("comfortable");
  const [showDock, setShowDock] = useState(true);

  return (
    <Menubar>
      <MenubarMenu>
        <MenubarTrigger>View</MenubarTrigger>
        <MenubarContent>
          <MenubarGroup>
            <MenubarLabel>Density</MenubarLabel>
            <MenubarRadioGroup value={density} onValueChange={setDensity}>
              <MenubarRadioItem value="comfortable">Comfortable</MenubarRadioItem>
              <MenubarRadioItem value="compact">Compact</MenubarRadioItem>
            </MenubarRadioGroup>
          </MenubarGroup>
          <MenubarSeparator />
          <MenubarCheckboxItem checked={showDock} onCheckedChange={setShowDock}>
            Show dock
          </MenubarCheckboxItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Go</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>Home</MenubarItem>
          <MenubarItem>Agents</MenubarItem>
          <MenubarItem>Tasks</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  );
}

/** Checkbox and radio selection state inside a menubar popup. */
export const SelectionStates: Story = {
  render: () => <StatefulMenubar />,
};
