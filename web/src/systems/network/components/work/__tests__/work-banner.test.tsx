// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const toastWarningMock = vi.fn();

vi.mock("sonner", () => ({
  toast: {
    warning: (...args: unknown[]) => toastWarningMock(...args),
  },
}));

import { WorkBanner, WORK_BANNER_HARD_STOP_THRESHOLD } from "../work-banner";

describe("WorkBanner auto-hide, tone, and hard-stop", () => {
  beforeEach(() => {
    toastWarningMock.mockReset();
  });

  it("Should render nothing when openCount is 0 from the start", () => {
    render(<WorkBanner hasNeedsInput={false} openCount={0} />);
    expect(screen.queryByTestId("network-work-banner")).toBeNull();
  });

  it("Should render the info tint for working-only state (no needs_input)", () => {
    render(<WorkBanner hasNeedsInput={false} openCount={2} />);
    const banner = screen.getByTestId("network-work-banner");
    expect(banner).toHaveAttribute("data-tone", "info");
    expect(banner).toHaveTextContent("2 active work in flight");
  });

  it("Should render the warning tint when work needs input (no solid signal fill)", () => {
    render(<WorkBanner hasNeedsInput openCount={3} />);
    const banner = screen.getByTestId("network-work-banner");
    expect(banner).toHaveAttribute("data-tone", "warning");
    expect(banner).toHaveTextContent("1 needs input · 2 working");
  });

  it("Should render the danger tint when `needsInputCount` crosses the hard-stop threshold", () => {
    render(
      <WorkBanner
        hasNeedsInput
        needsInputCount={WORK_BANNER_HARD_STOP_THRESHOLD + 1}
        openCount={WORK_BANNER_HARD_STOP_THRESHOLD + 2}
        workingCount={1}
      />
    );
    const banner = screen.getByTestId("network-work-banner");
    expect(banner).toHaveAttribute("data-tone", "danger");
  });

  it("Should fire a single <Sonner> toast when `needsInputCount` crosses the hard-stop threshold", () => {
    const { rerender } = render(
      <WorkBanner hasNeedsInput needsInputCount={2} openCount={3} workingCount={1} />
    );
    expect(toastWarningMock).not.toHaveBeenCalled();

    rerender(
      <WorkBanner
        hasNeedsInput
        needsInputCount={WORK_BANNER_HARD_STOP_THRESHOLD + 1}
        openCount={WORK_BANNER_HARD_STOP_THRESHOLD + 2}
        workingCount={1}
      />
    );
    expect(toastWarningMock).toHaveBeenCalledTimes(1);

    // Subsequent rerenders that stay beyond the threshold must NOT re-fire the toast.
    rerender(
      <WorkBanner
        hasNeedsInput
        needsInputCount={WORK_BANNER_HARD_STOP_THRESHOLD + 2}
        openCount={WORK_BANNER_HARD_STOP_THRESHOLD + 3}
        workingCount={1}
      />
    );
    expect(toastWarningMock).toHaveBeenCalledTimes(1);

    // Falling below the threshold rearms the trigger.
    rerender(<WorkBanner hasNeedsInput needsInputCount={1} openCount={2} workingCount={1} />);
    rerender(
      <WorkBanner
        hasNeedsInput
        needsInputCount={WORK_BANNER_HARD_STOP_THRESHOLD + 1}
        openCount={WORK_BANNER_HARD_STOP_THRESHOLD + 2}
        workingCount={1}
      />
    );
    expect(toastWarningMock).toHaveBeenCalledTimes(2);
  });

  it("Should interrupt an exit when work reopens", () => {
    const { rerender } = render(<WorkBanner hasNeedsInput={false} openCount={2} />);
    expect(screen.getByTestId("network-work-banner")).toBeInTheDocument();

    rerender(<WorkBanner hasNeedsInput={false} openCount={0} />);
    rerender(<WorkBanner hasNeedsInput={false} openCount={1} />);

    expect(screen.getByTestId("network-work-banner")).toHaveTextContent("1 active work in flight");
  });

  it("Should render the explicit breakdown when needsInputCount + workingCount are provided", () => {
    render(<WorkBanner hasNeedsInput needsInputCount={2} openCount={3} workingCount={1} />);
    const banner = screen.getByTestId("network-work-banner");
    expect(banner).toHaveTextContent("2 needs input · 1 working");
  });

  it("Should omit zero buckets in the breakdown", () => {
    render(<WorkBanner hasNeedsInput={false} needsInputCount={0} openCount={2} workingCount={2} />);
    expect(screen.getByTestId("network-work-banner")).toHaveTextContent("2 working");
  });
});
