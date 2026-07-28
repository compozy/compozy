import * as React from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "../sheet";
import { OverlayContainerContext } from "../hooks/use-overlay-container";
import { Button } from "../button";

type Side = "top" | "right" | "bottom" | "left";

function SheetExample({
  defaultOpen = false,
  side = "right",
}: {
  defaultOpen?: boolean;
  side?: Side;
}) {
  return (
    <Sheet defaultOpen={defaultOpen}>
      <SheetTrigger render={<Button>Open sheet</Button>} />
      <SheetContent side={side}>
        <SheetHeader>
          <SheetTitle>Configure agent</SheetTitle>
          <SheetDescription>Edit settings for the current workspace.</SheetDescription>
        </SheetHeader>
      </SheetContent>
    </Sheet>
  );
}

describe("Sheet", () => {
  it("Should open and render the configured side attribute", async () => {
    render(<SheetExample side="left" />);
    fireEvent.click(screen.getByRole("button", { name: "Open sheet" }));
    await waitFor(() => expect(screen.getByText("Configure agent")).toBeInTheDocument());
    const popup = screen.getByRole("dialog");
    expect(popup).toHaveAttribute("data-side", "left");
  }, 15_000);

  it.each<Side>(["top", "right", "bottom", "left"])(
    "Should reflect data-side='%s' when opened",
    async side => {
      render(<SheetExample defaultOpen side={side} />);
      await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
      expect(screen.getByRole("dialog")).toHaveAttribute("data-side", side);
    }
  );

  it("Should close on Escape", async () => {
    const user = userEvent.setup();
    render(<SheetExample defaultOpen />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should mount a sheet-overlay slot when the sheet is open", async () => {
    render(<SheetExample defaultOpen />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const overlay = document.body.querySelector(
      "[data-slot='sheet-overlay']"
    ) as HTMLElement | null;

    expect(overlay).not.toBeNull();
  });

  it("Should keep a window-scoped sheet open while a peer surface remains interactive", async () => {
    const user = userEvent.setup();

    function WindowHost() {
      const ref = React.useRef<HTMLDivElement | null>(null);
      const [container, setContainer] = React.useState<HTMLDivElement | null>(null);
      const [peerValue, setPeerValue] = React.useState("");
      React.useEffect(() => setContainer(ref.current), []);

      return (
        <>
          <div ref={ref} data-testid="os-window">
            {container ? (
              <OverlayContainerContext.Provider value={container}>
                <Sheet defaultOpen>
                  <SheetContent>
                    <SheetTitle>Window detail</SheetTitle>
                  </SheetContent>
                </Sheet>
              </OverlayContainerContext.Provider>
            ) : null}
          </div>
          <input
            aria-label="Peer composer"
            value={peerValue}
            onChange={event => setPeerValue(event.target.value)}
          />
        </>
      );
    }

    render(<WindowHost />);
    await waitFor(() => expect(screen.getByRole("dialog")).toBeInTheDocument());

    const windowEl = screen.getByTestId("os-window");
    const sheet = screen.getByRole("dialog");
    expect(windowEl.contains(sheet)).toBe(true);

    const peerComposer = screen.getByRole("textbox", { name: "Peer composer" });
    await user.click(peerComposer);
    expect(peerComposer).toHaveFocus();
    fireEvent.change(peerComposer, { target: { value: "still active" } });

    expect(peerComposer).toHaveValue("still active");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("Should throw when SheetContent is used outside <Sheet>", () => {
    const originalError = console.error;
    try {
      console.error = () => {};
      expect(() =>
        render(
          <SheetContent>
            <SheetTitle>orphan</SheetTitle>
          </SheetContent>
        )
      ).toThrow(/Sheet\.\* components must be used inside <Sheet>/);
    } finally {
      console.error = originalError;
    }
  });
});
