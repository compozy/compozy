import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";

describe("Collapsible", () => {
  it("Should render the collapsible root + trigger with stable data-slots", () => {
    const { container } = render(
      <Collapsible>
        <CollapsibleTrigger>Toggle</CollapsibleTrigger>
        <CollapsibleContent>Body</CollapsibleContent>
      </Collapsible>
    );
    expect(container.querySelector("[data-slot=collapsible]")).toBeInTheDocument();
    expect(container.querySelector("[data-slot=collapsible-trigger]")).toBeInTheDocument();
  });

  it("Should start closed and open the panel on trigger click", async () => {
    const user = userEvent.setup();
    render(
      <Collapsible>
        <CollapsibleTrigger>Toggle</CollapsibleTrigger>
        <CollapsibleContent>Body</CollapsibleContent>
      </Collapsible>
    );
    const trigger = screen.getByRole("button", { name: "Toggle" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Body")).toBeInTheDocument();
  });

  it("Should respect defaultOpen", () => {
    render(
      <Collapsible defaultOpen>
        <CollapsibleTrigger>Toggle</CollapsibleTrigger>
        <CollapsibleContent>Body</CollapsibleContent>
      </Collapsible>
    );
    expect(screen.getByRole("button", { name: "Toggle" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Body")).toBeInTheDocument();
  });

  it("Should preserve panel content in the DOM when closed with keepMounted", async () => {
    const user = userEvent.setup();
    render(
      <Collapsible defaultOpen>
        <CollapsibleTrigger>Toggle</CollapsibleTrigger>
        <CollapsibleContent keepMounted>Body</CollapsibleContent>
      </Collapsible>
    );
    expect(screen.getByText("Body")).toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: "Toggle" });
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("Body")).toBeInTheDocument();
    expect(screen.getByText("Body")).not.toBeVisible();
  });
});
