// Suite: Catalog empty disclosure row
// Invariant: review details stay collapsed until requested and row actions remain outside the opener.
// Boundary IN: the generic disclosure-row composition and Collapsible integration.
// Boundary OUT: task-template and automation-suggestion actions owned by their web component suites.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Clock3 } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { Button } from "../../button";
import { CatalogEmptyDisclosureRow } from "../catalog-empty-disclosure-row";

describe("CatalogEmptyDisclosureRow", () => {
  it("Should keep actions outside the opener and reveal details on demand", async () => {
    const user = userEvent.setup();
    const onUse = vi.fn();
    render(
      <ul>
        <CatalogEmptyDisclosureRow
          actions={
            <Button onClick={onUse} type="button" variant="neutral">
              Use suggestion
            </Button>
          }
          description="Runs every weekday after the stand-up."
          detail={<p>Schedule: 9:30 AM</p>}
          icon={<Clock3 className="size-3.5" />}
          title="Daily stand-up summary"
          tone="info"
        />
      </ul>
    );

    const opener = screen.getByRole("button", { name: /Daily stand-up summary/ });
    const action = screen.getByRole("button", { name: "Use suggestion" });

    expect(opener).not.toContainElement(action);
    expect(screen.queryByText("Schedule: 9:30 AM")).not.toBeInTheDocument();

    await user.click(opener);
    expect(screen.getByText("Schedule: 9:30 AM")).toBeVisible();

    await user.click(action);
    expect(onUse).toHaveBeenCalledTimes(1);
  });
});
