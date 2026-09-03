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
});
