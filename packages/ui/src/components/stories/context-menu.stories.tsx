import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "../context-menu";

const meta: Meta<typeof ContextMenu> = {
  title: "components/ContextMenu",
  component: ContextMenu,
  parameters: {
    docs: {
      description: {
        component:
          "Right-click destination chooser. Launch surfaces and deck tabs attach it to offer explicit open/close/pin choices; items follow the shared menu grammar (small-body rows, elevated focus, danger tint for destructive).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

async function openContextMenu(
  canvasElement: HTMLElement,
  triggerText: string,
  expectedItem: string
): Promise<void> {
  const trigger = within(canvasElement).getByText(triggerText);
  trigger.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true }));
  await waitFor(() => {
    const menu = within(document.body).getByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: expectedItem })).toBeInTheDocument();
  });
}

/** The window-tab menu contract: lifecycle verbs, pin, destructive bulk close. */
export const TabMenu: Story = {
  render: () => (
    <ContextMenu>
      <ContextMenuTrigger>
        <div className="grid h-24 w-64 place-items-center rounded-md border border-dashed border-line-strong text-small-body text-subtle select-none">
          Right-click a tab
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem>
          Close tab
          <ContextMenuShortcut>⌘W</ContextMenuShortcut>
        </ContextMenuItem>
        <ContextMenuItem>Close other tabs</ContextMenuItem>
        <ContextMenuItem>Close tabs to the right</ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem>Pin tab</ContextMenuItem>
        <ContextMenuItem>Move tab to new window</ContextMenuItem>
        <ContextMenuItem>Merge all windows</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  ),
  play: async ({ canvasElement }) => {
    await openContextMenu(canvasElement, "Right-click a tab", "Close tab");
  },
};

/** Launch-surface destinations with a disabled option explaining itself. */
export const DestinationMenu: Story = {
  render: () => (
    <ContextMenu>
      <ContextMenuTrigger>
        <div className="grid h-24 w-64 place-items-center rounded-md border border-dashed border-line-strong text-small-body text-subtle select-none">
          Right-click a dock app
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuLabel>Open Tasks</ContextMenuLabel>
        <ContextMenuItem>Open in new window</ContextMenuItem>
        <ContextMenuItem disabled>Open as tab in focused window</ContextMenuItem>
        <ContextMenuItem>Open new instance</ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuSub>
          <ContextMenuSubTrigger>Go to tab</ContextMenuSubTrigger>
          <ContextMenuSubContent>
            <ContextMenuItem>Tasks — Desktop 1</ContextMenuItem>
            <ContextMenuItem>Run #418 — Desktop 2</ContextMenuItem>
          </ContextMenuSubContent>
        </ContextMenuSub>
      </ContextMenuContent>
    </ContextMenu>
  ),
  play: async ({ canvasElement }) => {
    await openContextMenu(canvasElement, "Right-click a dock app", "Open in new window");
  },
};
