import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { History } from "lucide-react";
import { describe, expect, it } from "vitest";

import { LoopSection } from "../loop-section";

describe("LoopSection", () => {
  it("Should render the title in the header", () => {
    render(
      <LoopSection gist="3 events" icon={<History aria-hidden="true" />} title="History">
        <p>Section body</p>
      </LoopSection>
    );
    expect(screen.getByRole("button", { name: /History/ })).toBeInTheDocument();
  });

  it("Should keep the gist visible while the section is collapsed", () => {
    render(
      <LoopSection
        defaultOpen={false}
        gist="3 events"
        icon={<History aria-hidden="true" />}
        title="History"
      >
        <p>Section body</p>
      </LoopSection>
    );
    const trigger = screen.getByRole("button", { name: /History/ });
    expect(screen.getByText("3 events")).toBeVisible();
    expect(screen.getByText("History")).toBeVisible();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Section body")).not.toBeInTheDocument();
  });

  it("Should reveal and hide children when the header is toggled", async () => {
    const user = userEvent.setup();
    render(
      <LoopSection gist="3 events" icon={<History aria-hidden="true" />} title="History">
        <p>Section body</p>
      </LoopSection>
    );
    const trigger = screen.getByRole("button", { name: /History/ });
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Section body")).toBeVisible();
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("Section body")).not.toBeInTheDocument();
    await user.click(trigger);
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Section body")).toBeVisible();
  });

  it("Should keep the right slot outside any header button", async () => {
    const user = userEvent.setup();
    render(
      <LoopSection
        icon={<History aria-hidden="true" />}
        right={<button type="button">Open in builder</button>}
        title="History"
      >
        <p>Section body</p>
      </LoopSection>
    );
    const action = screen.getByRole("button", { name: "Open in builder" });
    expect(action.closest("[data-slot='collapsible-trigger']")).toBeNull();
    await user.click(action);
    expect(screen.getByRole("button", { name: /History/ })).toHaveAttribute(
      "aria-expanded",
      "true"
    );
    expect(screen.getByText("Section body")).toBeVisible();
  });
});
