import { render, screen, waitFor } from "@testing-library/react";
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

  it("Should list every wall-clock slot on a day the clock changes", () => {
    const zone = process.env.TZ;
    process.env.TZ = "America/New_York";
    try {
      // Fall back: a 25-hour day keeps 23:00 and 23:30.
      const fallBack = dateTimePickerSlots(new Date(2026, 10, 1), 30);
      expect(fallBack).toHaveLength(48);
      expect(fallBack.at(-1)?.getHours()).toBe(23);
      expect(fallBack.at(-1)?.getMinutes()).toBe(30);
      // Spring forward: 02:00 and 02:30 do not exist and are left out.
      const springForward = dateTimePickerSlots(new Date(2026, 2, 8), 30);
      expect(springForward).toHaveLength(46);
      expect(springForward.some(slot => slot.getHours() === 2)).toBe(false);
    } finally {
      process.env.TZ = zone;
    }
  });

  it("Should show an off-step value as the pressed time", async () => {
    const user = userEvent.setup();
    render(<Harness initial={new Date(2027, 2, 10, 11, 45)} />);
    await user.click(screen.getByRole("button", { name: /Mar/ }));
    const times = await screen.findByRole("group", { name: "Time" });
    const pressed = Array.from(times.querySelectorAll("button")).filter(
      button => button.getAttribute("aria-pressed") === "true"
    );
    expect(pressed).toHaveLength(1);
    expect(
      dateTimePickerSlots(new Date(2027, 2, 10), 30, new Date(2027, 2, 10, 11, 45))
    ).toHaveLength(49);
  });

  it("Should close a preset that resolves before the minimum", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    render(
      <Harness
        min={new Date(2027, 2, 15, 9, 0)}
        onValueChange={onValueChange}
        presets={[
          { label: "Tomorrow", value: new Date(2027, 2, 11, 9, 0) },
          { label: "Next week", value: new Date(2027, 2, 17, 9, 0) },
        ]}
      />
    );
    await user.click(screen.getByRole("button", { name: /Pick a date/ }));
    const tomorrow = await screen.findByRole("button", { name: "Tomorrow" });
    expect(tomorrow).toBeDisabled();
    await user.click(tomorrow);
    expect(onValueChange).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Next week" }));
    expect(onValueChange).toHaveBeenLastCalledWith(new Date(2027, 2, 17, 9, 0));
  });

  it("Should close the popover when the picker becomes disabled", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    const props = {
      onValueChange,
      presets: [{ label: "Tomorrow", value: new Date(2027, 2, 11, 9) }],
    };
    const { rerender } = render(
      <UIProvider>
        <DateTimePicker {...props} value={null} />
      </UIProvider>
    );
    await user.click(screen.getByRole("button", { name: /Pick a date/ }));
    expect(await screen.findByRole("button", { name: "Tomorrow" })).toBeInTheDocument();
    rerender(
      <UIProvider>
        <DateTimePicker {...props} disabled value={null} />
      </UIProvider>
    );
    await waitFor(() => expect(screen.queryByRole("button", { name: "Tomorrow" })).toBeNull());
    expect(onValueChange).not.toHaveBeenCalled();
  });
});
