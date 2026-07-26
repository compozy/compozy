import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Time } from "../time";

const FIXED_NOW = new Date("2026-05-11T12:00:00Z").getTime();

describe("Time", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(FIXED_NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should render <time dateTime> with relative text by default", () => {
    const past = new Date(FIXED_NOW - 5 * 60_000).toISOString();
    const { container } = render(<Time iso={past} />);
    const node = container.querySelector<HTMLTimeElement>('[data-slot="time"]');
    expect(node?.getAttribute("datetime")).toBe(past);
    expect(node?.dataset.mode).toBe("relative");
    expect(node?.textContent).toBe("5m ago");
    expect(node?.getAttribute("title")).toMatch(/2026/);
  });

  it("Should render the absolute timestamp when mode is absolute", () => {
    const past = new Date(FIXED_NOW - 5 * 60_000).toISOString();
    const { container } = render(<Time iso={past} mode="absolute" />);
    const node = container.querySelector<HTMLTimeElement>('[data-slot="time"]');
    expect(node?.dataset.mode).toBe("absolute");
    expect(node?.textContent).toMatch(/2026/);
    expect(node?.getAttribute("title")).toBe("5m ago");
  });

  it("Should refresh the rendered relative string after the 30s tick", () => {
    const start = new Date(FIXED_NOW - 5_000).toISOString();
    const { container } = render(<Time iso={start} />);
    const node = container.querySelector<HTMLTimeElement>('[data-slot="time"]');
    expect(node?.textContent).toBe("just now");

    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(node?.textContent).not.toBe("just now");
    expect(node?.textContent).toBe("1m ago");
  });

  it("Should render the — fallback for invalid ISO input", () => {
    const { container } = render(<Time iso="not-an-iso" />);
    const node = container.querySelector<HTMLTimeElement>('[data-slot="time"]');
    expect(node?.textContent).toBe("—");
  });
});
