import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "../dropdown-menu";

const meta: Meta<typeof DropdownMenu> = {
  title: "components/ui/DropdownMenu",
  component: DropdownMenu,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Action menu anchored to a trigger. Mix plain items, checkbox groups, radios, and nested submenus from a single composition.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const agentItems = [
  { value: "rename", label: "Rename session" },
  { value: "duplicate", label: "Duplicate session" },
  { value: "export", label: "Export transcript" },
] as const;

export const Default: Story = {
  render: () => (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="outline">Session actions</Button>} />
      <DropdownMenuContent>
        <DropdownMenuGroup>
          <DropdownMenuLabel>Session</DropdownMenuLabel>
          {agentItems.map(item => (
            <DropdownMenuItem key={item.value}>{item.label}</DropdownMenuItem>
          ))}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive">
          Delete
          <DropdownMenuShortcut>⌘⌫</DropdownMenuShortcut>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ),
};

function StatefulMenu() {
  const [lane, setLane] = useState("critical");
  const [autoScroll, setAutoScroll] = useState(true);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="outline">View options</Button>} />
      <DropdownMenuContent>
        <DropdownMenuGroup>
          <DropdownMenuLabel>Severity lane</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={lane} onValueChange={setLane}>
            <DropdownMenuRadioItem value="critical">Critical</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="warning">Warning</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="info">Info</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem checked={autoScroll} onCheckedChange={setAutoScroll}>
          Auto-scroll transcript
        </DropdownMenuCheckboxItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export const SelectionStates: Story = {
  render: () => <StatefulMenu />,
};

export const WithSubmenu: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Submenus open on hover or via the right arrow key. The primary menu keeps focus on its trigger while the sub-menu is active.",
      },
    },
  },
  render: () => (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="outline">Run</Button>} />
      <DropdownMenuContent>
        <DropdownMenuItem>Execute</DropdownMenuItem>
        <DropdownMenuSub>
          <DropdownMenuSubTrigger>Agent...</DropdownMenuSubTrigger>
          <DropdownMenuSubContent>
            <DropdownMenuItem>claude-code</DropdownMenuItem>
            <DropdownMenuItem>codex</DropdownMenuItem>
            <DropdownMenuItem>gemini-cli</DropdownMenuItem>
          </DropdownMenuSubContent>
        </DropdownMenuSub>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive">Cancel run</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  ),
};
