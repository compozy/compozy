import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

function PopoverExample({ defaultOpen = false }: { defaultOpen?: boolean }) {
  return (
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
  );
}

describe("Popover", () => {
  it("Should not render the content before the trigger is activated", () => {
    render(<PopoverExample />);
    expect(screen.queryByText("Filters")).not.toBeInTheDocument();
  });

  it("Should open on trigger click and render title + description", async () => {
    const user = userEvent.setup();
    render(<PopoverExample />);
    await user.click(screen.getByRole("button", { name: "Open popover" }));
    await waitFor(() => expect(screen.getByText("Filters")).toBeInTheDocument());
    expect(screen.getByText("Apply quick filters to the list.")).toBeInTheDocument();
  });

  it("Should close on Escape", async () => {
    const user = userEvent.setup();
    render(<PopoverExample defaultOpen />);
    await waitFor(() => expect(screen.getByText("Filters")).toBeInTheDocument());
    await user.keyboard("{Escape}");
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
