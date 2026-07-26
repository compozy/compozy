import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  Menubar,
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
} from "../menubar";

function Fixture() {
  return (
    <Menubar aria-label="System menus">
      <MenubarMenu>
        <MenubarTrigger>Session</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>New session</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Window</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>Minimize</MenubarItem>
          <MenubarSub>
            <MenubarSubTrigger>Move &amp; resize</MenubarSubTrigger>
            <MenubarSubContent>
              <MenubarItem>Left half</MenubarItem>
            </MenubarSubContent>
          </MenubarSub>
        </MenubarContent>
      </MenubarMenu>
      <MenubarMenu>
        <MenubarTrigger>Help</MenubarTrigger>
        <MenubarContent>
          <MenubarItem>Keyboard shortcuts…</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  );
}

describe("Menubar", () => {
  it("Should expose the bar as a menubar whose triggers are menu items", () => {
    render(<Fixture />);
    const menubar = screen.getByRole("menubar", { name: "System menus" });
    expect(menubar).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Session" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Window" })).toBeInTheDocument();
    expect(screen.getByRole("menuitem", { name: "Help" })).toBeInTheDocument();
  });

  it("Should move trigger focus with the right arrow key", async () => {
    const user = userEvent.setup();
    render(<Fixture />);
    await user.tab();
    expect(screen.getByRole("menuitem", { name: "Session" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("menuitem", { name: "Window" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("menuitem", { name: "Help" })).toHaveFocus();
  });

  it("Should switch the open menu when the pointer moves onto a sibling trigger", async () => {
    const user = userEvent.setup();
    render(<Fixture />);
    await user.click(screen.getByRole("menuitem", { name: "Session" }));
    expect(await screen.findByRole("menuitem", { name: "New session" })).toBeInTheDocument();
    // Pointer travel over an open menubar is carried by `mousemove` on the
    // sibling trigger; jsdom has no hit-testing, so drive that event directly.
    fireEvent.mouseMove(screen.getByRole("menuitem", { name: "Help" }));
    expect(
      await screen.findByRole("menuitem", { name: "Keyboard shortcuts…" })
    ).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "New session" })).not.toBeInTheDocument();
  });

  it("Should open a submenu from its trigger with the right arrow key", async () => {
    const user = userEvent.setup();
    render(<Fixture />);
    await user.click(screen.getByRole("menuitem", { name: "Window" }));
    expect(await screen.findByRole("menuitem", { name: "Minimize" })).toBeInTheDocument();
    await user.keyboard("{ArrowDown}{ArrowDown}");
    expect(screen.getByRole("menuitem", { name: "Move & resize" })).toHaveFocus();
    await user.keyboard("{ArrowRight}");
    expect(await screen.findByRole("menuitem", { name: "Left half" })).toBeInTheDocument();
  });
});
