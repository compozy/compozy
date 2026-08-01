import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuShortcut,
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
