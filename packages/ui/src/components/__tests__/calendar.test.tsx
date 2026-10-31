import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Calendar } from "../calendar";

describe("Calendar", () => {
  it("Should select an open day and mark it as the single selection", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const month = new Date(2027, 2, 1);
    const { rerender } = render(<Calendar mode="single" month={month} onSelect={onSelect} />);
    await user.click(screen.getByRole("button", { name: /March 10th, 2027/ }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    const picked = onSelect.mock.calls[0]?.[0] as Date;
    expect([picked.getFullYear(), picked.getMonth(), picked.getDate()]).toEqual([2027, 2, 10]);

    rerender(<Calendar mode="single" month={month} onSelect={onSelect} selected={picked} />);
    expect(screen.getByRole("button", { name: /March 10th, 2027/ })).toHaveAttribute(
      "data-selected-single",
      "true"
    );
  });

  it("Should refuse a disabled day", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <Calendar
        disabled={{ before: new Date(2027, 2, 15) }}
        mode="single"
        month={new Date(2027, 2, 1)}
        onSelect={onSelect}
      />
    );
    const closed = screen.getByRole("button", { name: /March 10th, 2027/ });
    expect(closed).toBeDisabled();
    await user.click(closed);
    expect(onSelect).not.toHaveBeenCalled();
  });
});
