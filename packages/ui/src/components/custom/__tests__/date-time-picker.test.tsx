import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

import { dateTimePickerDayValue, dateTimePickerSlots } from "../../../lib/date-time-picker";
import { DateTimePicker, type DateTimePickerProps } from "../date-time-picker";
import { UIProvider } from "../ui-provider";

function Harness(props: Partial<DateTimePickerProps> & { initial?: Date | null }) {
  const [value, setValue] = React.useState<Date | null>(props.initial ?? null);
  return (
    <UIProvider>
      <DateTimePicker
        {...props}
        onValueChange={next => {
          setValue(next);
          props.onValueChange?.(next);
        }}
        value={value}
      />
    </UIProvider>
  );
}

describe("DateTimePicker", () => {
  it("Should land a picked day on the default time, then take a picked time", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(<Harness min={new Date(2027, 2, 1, 12, 0)} onValueChange={onValueChange} />);
    await user.click(screen.getByRole("button", { name: /Pick a date/ }));
    await user.click(await screen.findByRole("button", { name: /March 10th, 2027/ }));
    expect(onValueChange).toHaveBeenLastCalledWith(new Date(2027, 2, 10, 9, 0));

    const times = screen.getByRole("group", { name: "Time" });
    const slot = Array.from(times.querySelectorAll("button")).find(
      button => button.getAttribute("aria-pressed") === "true"
    );
    expect(slot).toBeDefined();
    const later = times.querySelectorAll("button")[30] as HTMLButtonElement;
    await user.click(later);
    expect(onValueChange).toHaveBeenLastCalledWith(new Date(2027, 2, 10, 15, 0));
  });

  it("Should close times before the minimum and refuse earlier days", async () => {
    const user = userEvent.setup();
    render(<Harness initial={new Date(2027, 2, 10, 18, 0)} min={new Date(2027, 2, 10, 14, 10)} />);
    await user.click(screen.getByRole("button", { name: /Mar/ }));
    expect(await screen.findByRole("button", { name: /March 9th, 2027/ })).toBeDisabled();
    const slots = screen.getByRole("group", { name: "Time" }).querySelectorAll("button");
    expect(slots[28]).toBeDisabled(); // 14:00
    expect(slots[29]).toBeEnabled(); // 14:30
  });

  it("Should apply a preset and clear the value", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const tomorrow = new Date(2027, 2, 11, 9, 0);
    render(
      <Harness
        onValueChange={onValueChange}
        presets={[{ label: "Tomorrow", value: () => tomorrow }]}
      />
    );
    await user.click(screen.getByRole("button", { name: /Pick a date/ }));
    await user.click(await screen.findByRole("button", { name: "Tomorrow" }));
    expect(onValueChange).toHaveBeenLastCalledWith(tomorrow);
    await user.click(screen.getByRole("button", { name: "Clear date" }));
    expect(onValueChange).toHaveBeenLastCalledWith(null);
    expect(screen.getByRole("button", { name: /Pick a date/ })).toBeInTheDocument();
  });

  it("Should move a picked day past the minimum to its first open slot", () => {
    const min = new Date(2027, 2, 10, 14, 10);
    expect(
      dateTimePickerDayValue(new Date(2027, 2, 10), null, { min, step: 30, defaultTime: [9, 0] })
    ).toEqual(new Date(2027, 2, 10, 14, 30));
    expect(dateTimePickerSlots(new Date(2027, 2, 10), 30)).toHaveLength(48);
  });
});
