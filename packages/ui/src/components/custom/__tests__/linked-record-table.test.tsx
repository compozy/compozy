import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChevronRight } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { Pill } from "../pill";
import { LinkedRecordTable } from "../linked-record-table";

describe("LinkedRecordTable", () => {
  it("Should compose the section, table, title, pill, status, and open slots", () => {
    render(
      <LinkedRecordTable data-testid="linked-table" label="Child tasks">
        <LinkedRecordTable.Body>
          <LinkedRecordTable.Row data-testid="linked-row">
            <LinkedRecordTable.Cell className="w-8 pl-4">
              <Pill.Dot tone="success" />
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>
              <LinkedRecordTable.Title>
                <span>Provision bridge credentials</span>
                <span>
                  <Pill mono>TASK-004</Pill>
                  <Pill tone="success">done</Pill>
                </span>
              </LinkedRecordTable.Title>
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>Codex</LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>2m ago</LinkedRecordTable.Cell>
            <LinkedRecordTable.OpenCell>
              <a href="/tasks/task-004">
                Open <ChevronRight />
              </a>
            </LinkedRecordTable.OpenCell>
          </LinkedRecordTable.Row>
        </LinkedRecordTable.Body>
      </LinkedRecordTable>
    );

    expect(screen.getByTestId("linked-table")).toHaveAttribute("data-slot", "linked-record-table");
    expect(screen.getByTestId("linked-row")).toHaveAttribute(
      "data-slot",
      "linked-record-table-row"
    );
    expect(screen.getByText("Provision bridge credentials")).toBeInTheDocument();
    expect(screen.getByText("TASK-004")).toHaveAttribute("data-slot", "pill");
    expect(screen.getByRole("link", { name: /open/i })).toHaveAttribute("href", "/tasks/task-004");
  });

  it("Should render the empty slot when no body rows are provided", () => {
    render(<LinkedRecordTable label="Dependencies" empty={<p>No dependencies</p>} />);

    expect(screen.getByText("No dependencies")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("Should preserve interactive row actions without owning navigation state", async () => {
    const user = userEvent.setup();
    const onOpen = vi.fn();

    render(
      <LinkedRecordTable label="Runs">
        <LinkedRecordTable.Body>
          <LinkedRecordTable.Row>
            <LinkedRecordTable.Cell className="w-8 pl-4">
              <Pill.Dot tone="accent" />
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>
              <LinkedRecordTable.Title>run_01</LinkedRecordTable.Title>
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>attempt 1</LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>now</LinkedRecordTable.Cell>
            <LinkedRecordTable.OpenCell>
              <button type="button" onClick={onOpen}>
                Open
              </button>
            </LinkedRecordTable.OpenCell>
          </LinkedRecordTable.Row>
        </LinkedRecordTable.Body>
      </LinkedRecordTable>
    );

    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(onOpen).toHaveBeenCalledTimes(1);
  });
});
