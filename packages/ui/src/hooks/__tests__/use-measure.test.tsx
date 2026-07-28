import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useMeasure } from "../use-measure";

function MeasureHarness() {
  const [renderCount, setRenderCount] = useState(0);
  const [measureRef] = useMeasure<HTMLDivElement>();
  return (
    <div>
      <div ref={measureRef}>Measured</div>
      <button type="button" onClick={() => setRenderCount(count => count + 1)}>
        Rerender {renderCount}
      </button>
    </div>
  );
}

describe("useMeasure", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should keep one ResizeObserver attached across consumer rerenders", () => {
    const disconnect = vi.fn();
    const observe = vi.fn();
    const ResizeObserverMock = vi.fn(function ResizeObserverMock() {
      return { disconnect, observe, unobserve: vi.fn() };
    });
    vi.stubGlobal("ResizeObserver", ResizeObserverMock);

    render(<MeasureHarness />);
    expect(ResizeObserverMock).toHaveBeenCalledTimes(1);
    expect(observe).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Rerender 0" }));

    expect(screen.getByRole("button", { name: "Rerender 1" })).toBeInTheDocument();
    expect(ResizeObserverMock).toHaveBeenCalledTimes(1);
    expect(disconnect).not.toHaveBeenCalled();
  });
});
