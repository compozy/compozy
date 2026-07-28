// Suite: Settings number input
// Invariant: Controlled updates preserve the active editing element and accept consecutive digits.
// Boundary IN: Number input state synchronization and parent-controlled value updates.
// Boundary OUT: Settings persistence and daemon-side validation, owned by settings adapter suites.
import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { SettingsNumberInput } from "../settings-number-input";
import { SettingsAdvancedFold } from "../settings-advanced-fold";

describe("SettingsNumberInput", () => {
  it("Should preserve focus across controlled value updates", () => {
    function Harness() {
      const [value, setValue] = useState(1);
      return (
        <SettingsNumberInput aria-label="Retry limit" onValueChange={setValue} value={value} />
      );
    }

    render(<Harness />);
    const input = screen.getByRole("textbox", { name: "Retry limit" });
    input.focus();
    fireEvent.change(input, { target: { value: "12" } });

    expect(input).toHaveValue("12");
    expect(input).toHaveFocus();
  });

  it("Should reset a rejected raw value when the draft revision changes", () => {
    const { rerender } = render(
      <SettingsNumberInput
        aria-label="Retry limit"
        onValueChange={() => undefined}
        resetRevision={0}
        value={1}
      />
    );
    const input = screen.getByRole("textbox", { name: "Retry limit" });
    fireEvent.change(input, { target: { value: "invalid" } });
    expect(input).toHaveValue("invalid");

    rerender(
      <SettingsNumberInput
        aria-label="Retry limit"
        onValueChange={() => undefined}
        resetRevision={1}
        value={1}
      />
    );
    expect(input).toHaveValue("1");
    expect(input).not.toHaveAttribute("aria-invalid");
  });
});

describe("SettingsAdvancedFold", () => {
  it("Should keep invalid controls mounted when the fold closes", () => {
    render(
      <SettingsAdvancedFold defaultOpen>
        <SettingsNumberInput aria-label="Retry limit" onValueChange={() => undefined} value={1} />
      </SettingsAdvancedFold>
    );

    const input = screen.getByRole("textbox", { name: "Retry limit" });
    fireEvent.change(input, { target: { value: "invalid" } });
    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));

    expect(input).toBeInTheDocument();
    expect(input).not.toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Advanced" }));
    expect(input).toHaveValue("invalid");
    expect(input).toHaveAttribute("aria-invalid", "true");
  });
});
