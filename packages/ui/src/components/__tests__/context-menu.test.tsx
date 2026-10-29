import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Dialog, DialogContent, DialogTitle } from "../dialog";
import { describe, expect, it, vi } from "vitest";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "../context-menu";

function ContextMenuExample({ onClose, onPin }: { onClose?: () => void; onPin?: () => void }) {
  return (
    <ContextMenu>
      <ContextMenuTrigger>
        <div>Tab segment</div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onClick={onClose}>
          Close tab
          <ContextMenuShortcut>⌘W</ContextMenuShortcut>
        </ContextMenuItem>
        <ContextMenuItem disabled onClick={onPin}>
          Pin tab
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem variant="destructive">Close other tabs</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}

describe("ContextMenu", () => {
  it("Should preserve the parent item focus when a submenu opens on hover", async () => {
    const user = userEvent.setup();
    render(
      <ContextMenu>
        <ContextMenuTrigger>Project</ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuSub>
            <ContextMenuSubTrigger>Move</ContextMenuSubTrigger>
            <ContextMenuSubContent>
              <ContextMenuItem>Desktop 2</ContextMenuItem>
            </ContextMenuSubContent>
          </ContextMenuSub>
        </ContextMenuContent>
      </ContextMenu>
    );
    fireEvent.contextMenu(screen.getByText("Project"));
    const trigger = await screen.findByRole("menuitem", { name: "Move" });
    await user.keyboard("{ArrowDown}");
    expect(screen.queryByRole("menuitem", { name: "Desktop 2" })).not.toBeInTheDocument();
    const movedFocus: EventTarget[] = [];
    const recordFocus = (event: FocusEvent) => {
      if (event.target && event.target !== trigger) movedFocus.push(event.target);
    };
    document.addEventListener("focusin", recordFocus);
    try {
      await user.hover(trigger);
      await screen.findByRole("menuitem", { name: "Desktop 2" });
      expect(trigger).toHaveFocus();
      expect(movedFocus).toEqual([]);
    } finally {
      document.removeEventListener("focusin", recordFocus);
    }
  });

  it("Should transfer focus before hiding its trigger inside a dialog", async () => {
    render(
      <Dialog defaultOpen>
        <DialogContent>
          <DialogTitle>Projects</DialogTitle>
          <ContextMenuExample />
        </DialogContent>
      </Dialog>
    );
    const trigger = screen
      .getByText("Tab segment")
      .closest<HTMLElement>("[data-slot=context-menu-trigger]")!;
    trigger.tabIndex = 0;
    trigger.focus();
    const hiddenFocus: Element[] = [];
    const setAttribute = Element.prototype.setAttribute;
    const writes = vi.spyOn(Element.prototype, "setAttribute").mockImplementation(function (
      this: Element,
      name: string,
      value: string
    ) {
      const active = this.ownerDocument.activeElement;
      if ((name === "aria-hidden" && value === "true") || name === "inert") {
        if (active && this.contains(active)) hiddenFocus.push(active);
      }
      setAttribute.call(this, name, value);
    });
    try {
      fireEvent.contextMenu(trigger);
      await waitFor(() => expect(screen.getByRole("menu")).toHaveFocus());
      expect(hiddenFocus).toEqual([]);
    } finally {
      writes.mockRestore();
    }
  });

  it("Should stay closed until the trigger receives a contextmenu event", () => {
    render(<ContextMenuExample />);
    expect(screen.queryByText("Close tab")).not.toBeInTheDocument();
  });

  it("Should open on contextmenu and fire the selected item's handler [UT-170]", async () => {
    const onClose = vi.fn();
    render(<ContextMenuExample onClose={onClose} />);

    fireEvent.contextMenu(screen.getByText("Tab segment"));

    await waitFor(() => expect(screen.getByText("Close tab")).toBeInTheDocument());
    expect(screen.getByText("⌘W")).toBeInTheDocument();
    expect(screen.getByText("Close other tabs")).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByText("Close tab"));

    expect(onClose).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.queryByText("Close tab")).not.toBeInTheDocument());
  });

  it("Should keep disabled items inert and reachable as disabled to assistive tech", async () => {
    const onPin = vi.fn();
    render(<ContextMenuExample onPin={onPin} />);

    fireEvent.contextMenu(screen.getByText("Tab segment"));
    await waitFor(() => expect(screen.getByText("Pin tab")).toBeInTheDocument());

    const pinItem = screen.getByRole("menuitem", { name: "Pin tab" });
    expect(pinItem).toHaveAttribute("aria-disabled", "true");
    expect(pinItem).toHaveAttribute("data-disabled");
    fireEvent.click(pinItem);
    expect(onPin).not.toHaveBeenCalled();
  });
});
