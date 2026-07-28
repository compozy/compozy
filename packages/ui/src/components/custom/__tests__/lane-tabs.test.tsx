import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

import { LaneTabs, type LaneTabsItem } from "../lane-tabs";

type LaneValue = "all" | "active" | "done";

const ITEMS: ReadonlyArray<LaneTabsItem<LaneValue>> = [
  { value: "all", label: "All", count: 12 },
  { value: "active", label: "Active", count: 4 },
  { value: "done", label: "Done", count: 8 },
] as const;

function Harness({ initial = "all" }: { initial?: LaneValue }) {
  const [value, setValue] = React.useState<LaneValue>(initial);
  return <LaneTabs items={ITEMS} value={value} onChange={setValue} ariaLabel="Lanes" />;
}

describe("LaneTabs", () => {
  it("Should render counts and mark the active tab as aria-selected", () => {
    render(<Harness initial="active" />);
    const active = screen.getByRole("tab", { name: /active/i });
    expect(active).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
  });

  it("Should advance the selection on ArrowRight and wrap on End", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<LaneTabs items={ITEMS} value="all" onChange={onChange} ariaLabel="Lanes" />);
    await user.click(screen.getByRole("tab", { name: /all/i }));
    onChange.mockClear();
    await user.keyboard("{ArrowRight}");
    expect(onChange).toHaveBeenLastCalledWith("active");
    await user.keyboard("{End}");
    expect(onChange).toHaveBeenLastCalledWith("done");
  });

  it("Should retreat with ArrowLeft and Home", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<LaneTabs items={ITEMS} value="active" onChange={onChange} ariaLabel="Lanes" />);
    await user.click(screen.getByRole("tab", { name: /active/i }));
    onChange.mockClear();
    await user.keyboard("{ArrowLeft}");
    expect(onChange).toHaveBeenLastCalledWith("all");
    await user.keyboard("{Home}");
    expect(onChange).toHaveBeenLastCalledWith("all");
  });
});
