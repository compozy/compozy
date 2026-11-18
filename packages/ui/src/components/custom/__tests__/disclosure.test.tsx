import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Disclosure } from "../disclosure";

describe("Disclosure", () => {
  it("Should start closed and reveal the panel on toggle", async () => {
    const user = userEvent.setup();
    render(
      <Disclosure label="More options">
        <p>Hidden field</p>
      </Disclosure>
    );

    const toggle = screen.getByRole("button", { name: "More options" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Hidden field")).toBeNull();

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Hidden field")).toBeVisible();
  });

  it("Should keep a closed panel mounted when asked", () => {
    render(
      <Disclosure label="Advanced" keepMounted>
        <input aria-label="Limit" defaultValue="5" />
      </Disclosure>
    );
    expect(screen.getByLabelText("Limit", { selector: "input" })).toBeInTheDocument();
  });

  it("Should keep a closed panel findable when asked, without a keepMounted warning", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const user = userEvent.setup();
    render(
      <Disclosure label="Response schema" hiddenUntilFound>
        <p id="schema-field">session_id</p>
      </Disclosure>
    );
    const panel = screen.getByText("session_id").closest("[data-slot='disclosure-content']");
    expect(panel).toHaveAttribute("hidden", "until-found");
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();

    await user.click(screen.getByRole("button", { name: "Response schema" }));
    expect(panel).not.toHaveAttribute("hidden");
  });

  it("Should forward test ids and honor controlled state", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <Disclosure
        label="Advanced"
        variant="framed"
        open
        onOpenChange={onOpenChange}
        data-testid="fold"
        triggerProps={{ "data-testid": "fold-toggle" }}
        contentProps={{ id: "fold-body" }}
      >
        <p>Body</p>
      </Disclosure>
    );

    expect(screen.getByTestId("fold")).toHaveAttribute("data-variant", "framed");
    expect(screen.getByText("Body").closest("#fold-body")).not.toBeNull();
    await user.click(screen.getByTestId("fold-toggle"));
    expect(onOpenChange).toHaveBeenCalledWith(false, expect.anything());
  });
});
