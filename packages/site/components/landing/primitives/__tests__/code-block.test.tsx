import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LandingCodeBlock } from "../code-block";

async function clickCopyButton() {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Copy to clipboard" }));
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("LandingCodeBlock", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("shows copy success and then returns to idle", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    render(<LandingCodeBlock code="agh daemon start" />);

    await clickCopyButton();

    expect(writeText).toHaveBeenCalledWith("agh daemon start");
    expect(screen.getByRole("button", { name: "Copied" })).toBeDefined();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(screen.getByRole("button", { name: "Copy to clipboard" })).toBeDefined();
  });

  it("shows copy failure instead of swallowing clipboard errors", async () => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockRejectedValue(new Error("blocked")) },
      configurable: true,
    });

    render(<LandingCodeBlock code="agh daemon start" />);

    await clickCopyButton();

    expect(screen.getByRole("button", { name: "Copy failed" })).toBeDefined();
  });
});
