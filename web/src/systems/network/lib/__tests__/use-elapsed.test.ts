// @vitest-environment jsdom

import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { formatElapsedSeconds, useElapsedSeconds } from "../use-elapsed";

describe("formatElapsedSeconds", () => {
  it("returns empty string for null", () => {
    expect(formatElapsedSeconds(null)).toBe("");
  });

  it("formats seconds under a minute", () => {
    expect(formatElapsedSeconds(0)).toBe("0s");
    expect(formatElapsedSeconds(45)).toBe("45s");
  });

  it("formats minutes and remainder", () => {
    expect(formatElapsedSeconds(60)).toBe("1m");
    expect(formatElapsedSeconds(125)).toBe("2m 5s");
  });

  it("formats hours and remainder", () => {
    expect(formatElapsedSeconds(3_600)).toBe("1h");
    expect(formatElapsedSeconds(3_660)).toBe("1h 1m");
  });
});

describe("useElapsedSeconds", () => {
  it("returns null when start is null", () => {
    const { result } = renderHook(() => useElapsedSeconds(null));
    expect(result.current).toBeNull();
  });

  it("returns null when start is invalid", () => {
    const { result } = renderHook(() => useElapsedSeconds("not-a-date"));
    expect(result.current).toBeNull();
  });

  it("returns a non-negative second count for a recent timestamp", () => {
    const { result } = renderHook(() => useElapsedSeconds(new Date()));
    expect(result.current).toBeGreaterThanOrEqual(0);
  });
});
