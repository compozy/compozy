// Invariant: Catalog and Activity are sibling Agents locations, one click apart.
// Owning layer: AgentsAppTabs. Canonical suite: this file.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AgentsAppTabs } from "../agents-app-tabs";

const navigate = vi.fn();

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => navigate,
}));

describe("AgentsAppTabs", () => {
  beforeEach(() => {
    navigate.mockReset();
  });

  it("Should mark the current location and open the other in one click", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<AgentsAppTabs value="catalog" />);

    expect(screen.getByTestId("agents-tab-catalog")).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tablist", { name: "Agents app locations" })).toBeInTheDocument();

    await user.click(screen.getByTestId("agents-tab-activity"));
    expect(navigate).toHaveBeenCalledWith({ to: "/agents/activity" });

    rerender(<AgentsAppTabs value="activity" />);
    expect(screen.getByTestId("agents-tab-activity")).toHaveAttribute("aria-selected", "true");

    await user.click(screen.getByTestId("agents-tab-catalog"));
    expect(navigate).toHaveBeenCalledWith({ to: "/agents" });
  });
});
