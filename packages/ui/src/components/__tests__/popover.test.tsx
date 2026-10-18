import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { describe, expect, it } from "vitest";

import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from "../popover";
import { Button } from "../button";
import { UIProvider } from "../custom/ui-provider";

function PopoverExample({ defaultOpen = false }: { defaultOpen?: boolean }) {
  return (
    <UIProvider reducedMotion="never">
      <Popover defaultOpen={defaultOpen}>
        <PopoverTrigger render={<Button>Open popover</Button>} />
        <PopoverContent side="bottom" align="start">
          <PopoverHeader>
            <PopoverTitle>Filters</PopoverTitle>
            <PopoverDescription>Apply quick filters to the list.</PopoverDescription>
          </PopoverHeader>
          <input aria-label="query" defaultValue="" />
        </PopoverContent>
      </Popover>
    </UIProvider>
  );
}

describe("Popover", () => {
  it("Should close on Escape", async () => {
    const user = userEvent.setup();
    render(<PopoverExample defaultOpen />);
    await waitFor(() => expect(screen.getByText("Filters")).toBeInTheDocument());
    await user.keyboard("{Escape}");

    const exitingPopup = document.querySelector('[data-slot="popover-content"]');
    expect(exitingPopup).toHaveAttribute("aria-hidden", "true");
    expect(exitingPopup).toHaveAttribute("inert");

    await waitFor(() => expect(screen.queryByText("Filters")).not.toBeInTheDocument(), {
      timeout: 1500,
    });
  });

  it("Should throw when PopoverContent is rendered outside <Popover>", () => {
    const originalError = console.error;
    try {
      console.error = () => {};
      expect(() =>
        render(
          <PopoverContent>
            <PopoverTitle>orphan</PopoverTitle>
          </PopoverContent>
        )
      ).toThrow(/Popover\.\* components must be used inside <Popover>/);
    } finally {
      console.error = originalError;
    }
  });

  // Invariant: a pointer open never rings the popover's first control; a keyboard
  // open still lands on it, and an autofocused or consumer-named target wins.
  describe("initial focus", () => {
    const popup = () => document.querySelector<HTMLElement>('[data-slot="popover-content"]');

    it("Should focus the popup, not its first control, on a pointer open", async () => {
      const user = userEvent.setup();
      render(<PopoverExample />);
      expect(screen.queryByText("Filters")).not.toBeInTheDocument();
      await user.click(screen.getByRole("button", { name: "Open popover" }));
      await waitFor(() => expect(screen.getByText("Filters")).toBeInTheDocument());
      expect(screen.getByText("Apply quick filters to the list.")).toBeInTheDocument();
      await waitFor(() => expect(popup()).toHaveFocus());
      expect(screen.getByRole("textbox", { name: "query" })).not.toHaveFocus();
    });

    it("Should focus the first control on a keyboard open", async () => {
      const user = userEvent.setup();
      render(<PopoverExample />);
      await user.tab();
      await user.keyboard("{Enter}");
      await waitFor(() => expect(screen.getByRole("textbox", { name: "query" })).toHaveFocus());
    });

    it("Should keep an autofocused control on a pointer open", async () => {
      const user = userEvent.setup();
      render(
        <UIProvider reducedMotion="never">
          <Popover>
            <PopoverTrigger render={<Button>Open</Button>} />
            <PopoverContent>
              <Button>First</Button>
              <input aria-label="search" autoFocus />
            </PopoverContent>
          </Popover>
        </UIProvider>
      );
      await user.click(screen.getByRole("button", { name: "Open" }));
      await waitFor(() => expect(screen.getByRole("textbox", { name: "search" })).toHaveFocus());
    });

    it("Should honor a consumer initialFocus", async () => {
      const user = userEvent.setup();
      function Harness() {
        const target = React.useRef<HTMLButtonElement | null>(null);
        return (
          <UIProvider reducedMotion="never">
            <Popover>
              <PopoverTrigger render={<Button>Open</Button>} />
              <PopoverContent initialFocus={target}>
                <Button>First</Button>
                <Button ref={target}>Second</Button>
              </PopoverContent>
            </Popover>
          </UIProvider>
        );
      }
      render(<Harness />);
      await user.click(screen.getByRole("button", { name: "Open" }));
      await waitFor(() => expect(screen.getByRole("button", { name: "Second" })).toHaveFocus());
    });
  });

  it("Should call onOpenChange when trigger toggles", async () => {
    const user = userEvent.setup();
    const calls: boolean[] = [];
    render(
      <Popover onOpenChange={next => calls.push(next)}>
        <PopoverTrigger render={<Button>Open</Button>} />
        <PopoverContent>
          <PopoverTitle>hello</PopoverTitle>
        </PopoverContent>
      </Popover>
    );
    await user.click(screen.getByRole("button", { name: "Open" }));
    await waitFor(() => expect(calls).toContain(true));
  });
});
