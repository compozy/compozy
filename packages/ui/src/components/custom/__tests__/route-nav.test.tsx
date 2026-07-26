import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RouteNav } from "../route-nav";

describe("RouteNav", () => {
  it("Should render a labeled nav of real links with aria-current on the active route", () => {
    render(
      <RouteNav aria-label="Tasks views">
        <RouteNav.Link aria-current="page" href="#list">
          List
        </RouteNav.Link>
        <RouteNav.Link href="#kanban">Kanban</RouteNav.Link>
        <RouteNav.Link href="#inbox">
          Inbox <RouteNav.Count data-testid="inbox-count">2</RouteNav.Count>
        </RouteNav.Link>
      </RouteNav>
    );

    expect(screen.getByRole("navigation", { name: "Tasks views" })).toBeInTheDocument();
    const active = screen.getByRole("link", { name: "List" });
    expect(active).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "Kanban" })).not.toHaveAttribute("aria-current");
    expect(screen.getByTestId("inbox-count")).toHaveTextContent("2");
  });

  it("Should support a custom render element for router links", () => {
    render(
      <RouteNav aria-label="Views">
        <RouteNav.Link render={<button type="button" />}>Kanban</RouteNav.Link>
      </RouteNav>
    );
    expect(screen.getByRole("button", { name: "Kanban" })).toBeInTheDocument();
  });
});
