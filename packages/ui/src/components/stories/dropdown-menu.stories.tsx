import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Copy, FolderInput, Hash, Pencil, Trash2 } from "../../icons";
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
import type { MenuTreeNode } from "../../lib/menu-tree";
import { MenuTreeItems } from "../menu-tree";

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

const sessionTree: MenuTreeNode[] = [
  { kind: "item", id: "rename", label: "Rename session", icon: Pencil },
  {
    kind: "sub",
    id: "copy",
    label: "Copy",
    icon: Copy,
    children: [
      { kind: "item", id: "copy-id", label: "Copy session ID", icon: Hash },
      { kind: "item", id: "copy-title", label: "Copy title", icon: Copy },
    ],
  },
  {
    kind: "sub",
    id: "move",
    label: "Move",
    icon: FolderInput,
    children: [
      {
        kind: "group",
        id: "folders",
        label: "Folder",
        children: [
          { kind: "item", id: "specs", label: "specs", checked: true },
          { kind: "item", id: "fixes", label: "fix-patches" },
        ],
      },
    ],
  },
  { kind: "separator", id: "end" },
  { kind: "item", id: "delete", label: "Delete session", icon: Trash2, destructive: true },
];

export const MenuTree: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "One `MenuTreeNode` tree renders the same rows in a dropdown or a context menu: frequent verbs at the top, related verbs behind a submenu, a group label inside a submenu, destructive last.",
      },
    },
  },
  render: () => (
    <DropdownMenu>
      <DropdownMenuTrigger render={<Button variant="outline">Session</Button>} />
      <DropdownMenuContent className="w-auto min-w-56">
        <MenuTreeItems host="dropdown" nodes={sessionTree} />
      </DropdownMenuContent>
    </DropdownMenu>
  ),
};

/** A virtual element at a point: the menu opens where the pointer was, with no trigger. */
function pointAnchor(x: number, y: number) {
  return { getBoundingClientRect: () => DOMRect.fromRect({ x, y, width: 0, height: 0 }) };
}

export const AtPoint: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "A controlled menu anchored to a point instead of a trigger, for a context click that a host relays from a surface that cannot nest its own menu.",
      },
    },
  },
  render: function AtPointStory() {
    const [point, setPoint] = useState<{ x: number; y: number } | null>(null);
    return (
      <>
        <div
          className="grid h-40 w-72 place-items-center rounded-lg border border-dashed border-line text-small-body text-muted-foreground"
          onContextMenu={event => {
            event.preventDefault();
            setPoint({ x: event.clientX, y: event.clientY });
          }}
        >
          Right-click anywhere here
        </div>
        <DropdownMenu open={point !== null} onOpenChange={open => !open && setPoint(null)}>
          <DropdownMenuContent
            anchor={point ? pointAnchor(point.x, point.y) : undefined}
            side="bottom"
            align="start"
            sideOffset={0}
          >
            <DropdownMenuItem>Archive project</DropdownMenuItem>
            <DropdownMenuItem variant="destructive">Remove project</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </>
    );
  },
};
