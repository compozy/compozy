import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../table";

describe("Table", () => {
  it("Should default the container to sideways scroll", () => {
    render(
      <Table aria-label="Sessions">
        <TableHeader>
          <TableRow>
            <TableHead>ID</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>sess_01</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    );

    const table = screen.getByRole("table", { name: "Sessions" });
    expect(table.parentElement).toHaveAttribute("data-overflow-x", "auto");
  });

  it("Should let a fixed layout opt out of sideways scroll", () => {
    render(
      <Table aria-label="Command journal" overflowX="hidden">
        <TableHeader>
          <TableRow>
            <TableHead>Command</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>make gate</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    );

    const table = screen.getByRole("table", { name: "Command journal" });
    expect(table.parentElement).toHaveAttribute("data-overflow-x", "hidden");
  });
});
