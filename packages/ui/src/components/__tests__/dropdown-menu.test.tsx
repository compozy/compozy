import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

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
  DropdownMenuTrigger,
} from "../dropdown-menu";
import { tidyMenuTree, type MenuTreeNode } from "../../lib/menu-tree";
import { MenuTreeItems } from "../menu-tree";

describe("DropdownMenu", () => {
  it("Should mount closed and expose a trigger with the stable data-slot", () => {
    const { container } = render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Rename</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    expect(container.querySelector("[data-slot=dropdown-menu-trigger]")).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Rename" })).not.toBeInTheDocument();
  });

  it("Should reveal items after the trigger is clicked", async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuGroup>
            <DropdownMenuLabel>Session</DropdownMenuLabel>
            <DropdownMenuItem>Rename</DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive">
            Delete
            <DropdownMenuShortcut>⌘⌫</DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(await screen.findByRole("menuitem", { name: "Rename" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: /Delete/ })).toBeInTheDocument();
  });

  it("Should fire onClick when a plain item is selected", async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={handleClick}>Rename</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));
    expect(handleClick).toHaveBeenCalledTimes(1);
    expect(handleClick).toHaveBeenCalled();
  });

  it("Should toggle a checkbox item via onCheckedChange", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuCheckboxItem checked={false} onCheckedChange={handleChange}>
            Auto-scroll
          </DropdownMenuCheckboxItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("menuitemcheckbox", { name: "Auto-scroll" }));
    expect(handleChange).toHaveBeenCalledTimes(1);
    expect(handleChange).toHaveBeenCalledWith(true, expect.anything());
  });

  it("Should update the radio group value when a radio item is selected", async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuRadioGroup value="critical" onValueChange={handleChange}>
            <DropdownMenuRadioItem value="critical">Critical</DropdownMenuRadioItem>
            <DropdownMenuRadioItem value="warning">Warning</DropdownMenuRadioItem>
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("menuitemradio", { name: "Warning" }));
    expect(handleChange).toHaveBeenCalledTimes(1);
    expect(handleChange).toHaveBeenCalledWith("warning", expect.anything());
  });

  it("Should dismiss on Escape and return focus to the trigger", async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Rename</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    expect(await screen.findByRole("menuitem", { name: "Rename" })).toBeInTheDocument();

    await user.keyboard("{Escape}");

    await waitFor(() => expect(screen.queryByRole("menuitem", { name: "Rename" })).toBeNull(), {
      timeout: 1500,
    });
    expect(trigger).toHaveFocus();
  });

  it("Should dismiss on outside press", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button type="button">Outside</button>
        <DropdownMenu>
          <DropdownMenuTrigger>Open</DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem>Rename</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(await screen.findByRole("menuitem", { name: "Rename" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Outside" }));

    await waitFor(() => expect(screen.queryByRole("menuitem", { name: "Rename" })).toBeNull(), {
      timeout: 1500,
    });
  });

  it("Should return focus to the trigger after an item is chosen", async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Rename</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    const trigger = screen.getByRole("button", { name: "Open" });
    await user.click(trigger);
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));

    await waitFor(() => expect(screen.queryByRole("menuitem", { name: "Rename" })).toBeNull(), {
      timeout: 1500,
    });
    expect(trigger).toHaveFocus();
  });

  it("Should open a controlled menu at a virtual point anchor without a trigger", async () => {
    const anchor = {
      getBoundingClientRect: () => DOMRect.fromRect({ x: 120, y: 80, width: 0, height: 0 }),
    };
    render(
      <DropdownMenu open>
        <DropdownMenuContent anchor={anchor}>
          <DropdownMenuItem>Archive project</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
    expect(await screen.findByRole("menuitem", { name: "Archive project" })).toBeInTheDocument();
    // The positioner places the popup from the virtual point, not from a trigger at the origin.
    const positioner = screen.getByRole("menu").parentElement!;
    // The zero-height test viewport flips placement above the point: y=80 minus sideOffset=4.
    await waitFor(() => {
      expect(screen.getByRole("menu")).toHaveAttribute("data-side", "top");
      expect(positioner.style.transform).toBe("translate(120px, 76px)");
    });
  });
});

describe("MenuTreeItems", () => {
  const tree = (onRename: () => void): MenuTreeNode[] => [
    { kind: "item", id: "rename", label: "Rename", onSelect: onRename },
    {
      kind: "sub",
      id: "move",
      label: "Move",
      children: [
        {
          kind: "group",
          id: "folders",
          label: "Folder",
          children: [
            { kind: "item", id: "specs", label: "specs", checked: true },
            { kind: "item", id: "docs", label: "docs", checked: false },
          ],
        },
      ],
    },
    { kind: "separator", id: "end" },
    { kind: "item", id: "delete", label: "Delete", destructive: true },
  ];

  it("Should render items, open a submenu with its group, and select an item", async () => {
    const user = userEvent.setup();
    const onRename = vi.fn();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <MenuTreeItems host="dropdown" nodes={tree(onRename)} />
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("menuitem", { name: "Move" }));
    expect(await screen.findByRole("group", { name: "Folder" })).toBeInTheDocument();
    // Members of a set announce which one is current, not only through the check glyph.
    expect(screen.getByRole("menuitemradio", { name: "specs" })).toBeChecked();
    expect(screen.getByRole("menuitemradio", { name: "docs" })).not.toBeChecked();
    expect(screen.getByRole("menuitem", { name: "Delete" })).toHaveAttribute(
      "data-variant",
      "destructive"
    );
    await user.click(screen.getByRole("menuitem", { name: "Rename" }));
    expect(onRename).toHaveBeenCalledOnce();
  });

  it("Should keep the menu open when an informational item sets closeOnClick false", async () => {
    const user = userEvent.setup();
    render(
      <DropdownMenu>
        <DropdownMenuTrigger>Open</DropdownMenuTrigger>
        <DropdownMenuContent>
          <MenuTreeItems
            host="dropdown"
            nodes={[
              { kind: "item", id: "why", label: "Moves with its parent", closeOnClick: false },
            ]}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("menuitem", { name: "Moves with its parent" }));
    expect(screen.getByRole("menuitem", { name: "Moves with its parent" })).toBeInTheDocument();
  });

  it("Should drop empty submenus and stray separators", () => {
    const tidy = tidyMenuTree([
      { kind: "separator", id: "lead" },
      { kind: "item", id: "rename", label: "Rename" },
      { kind: "separator", id: "a" },
      { kind: "separator", id: "b" },
      { kind: "sub", id: "empty", label: "Move", children: [{ kind: "separator", id: "c" }] },
      { kind: "item", id: "delete", label: "Delete" },
      { kind: "separator", id: "trail" },
    ]);
    expect(tidy.map(node => node.id)).toEqual(["rename", "a", "delete"]);
  });
});
