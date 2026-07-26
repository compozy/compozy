import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { BlogCodeBlock } from "../code-block";

async function clickCopyButton(name: string | RegExp = "Copy code") {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name }));
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("BlogCodeBlock", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("copies rendered code and then returns to idle", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    render(
      <BlogCodeBlock data-language="bash">
        <code>agh daemon start</code>
      </BlogCodeBlock>
    );

    await clickCopyButton();

    expect(writeText).toHaveBeenCalledWith("agh daemon start");
    expect(screen.getByRole("button", { name: "Copied" })).toBeDefined();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(screen.getByRole("button", { name: "Copy code" })).toBeDefined();
  });

  it("shows copy failure when the browser blocks clipboard access", async () => {
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText: vi.fn().mockRejectedValue(new Error("blocked")) },
      configurable: true,
    });

    render(
      <BlogCodeBlock data-language="bash">
        <code>agh daemon start</code>
      </BlogCodeBlock>
    );

    await clickCopyButton();

    expect(screen.getByRole("button", { name: "Copy failed" })).toBeDefined();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(screen.getByRole("button", { name: "Copy code" })).toBeDefined();
  });
});
