import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Button } from "../button";
import { ListGroup } from "../custom/list-group";

describe("ListGroup", () => {
  it("Should render the label, count chip, and body children", () => {
    render(
      <ListGroup count={3} data-testid="list-group" label="Global">
        <div data-testid="list-row">Operator Style</div>
      </ListGroup>
    );

    const group = screen.getByTestId("list-group");
    expect(group).toHaveAttribute("data-slot", "list-group");
    expect(within(group).getByText("Global")).toHaveAttribute("data-slot", "list-group-label");
    expect(within(group).getByText("3")).toHaveAttribute("data-slot", "pill");
    expect(within(group).getByTestId("list-row")).toHaveTextContent("Operator Style");
  });

  it("Should expose an actions slot that is keyboard reachable", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <ListGroup
        actions={
          <Button data-testid="group-action" onClick={onClick} size="sm" type="button">
            Refresh
          </Button>
        }
        count={1}
        label="Workspace"
      >
        <div>Launch Brief</div>
      </ListGroup>
    );

    await user.tab();
    expect(screen.getByTestId("group-action")).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("Should render compound Header and Items parts directly", () => {
    render(
      <ListGroup.Root data-testid="manual-list-group">
        <ListGroup.Header count={2} label="Agent" />
        <ListGroup.Items data-testid="manual-items">
          <div>CTO Tone</div>
        </ListGroup.Items>
      </ListGroup.Root>
    );

    expect(screen.getByTestId("manual-list-group")).toHaveAttribute("data-slot", "list-group");
    expect(screen.getByTestId("manual-items")).toHaveAttribute("data-slot", "list-group-items");
  });
});
