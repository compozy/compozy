// Suite: task result section
// Invariant: external results disclose one bounded page only after an explicit user action and
// keep page navigation and whole-result copy reachable by accessible controls.
// Owning layer: task result presentation. Boundary OUT: query behavior belongs to the hook suite.
import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { TaskResultPageController } from "../task-result-section";
import { TaskResultSection } from "../task-result-section";

function ExternalResultFixture({ onCopy }: { onCopy: () => Promise<void> }) {
  const [open, setOpen] = useState(false);
  const controller: TaskResultPageController = {
    canGoNext: true,
    canGoPrevious: false,
    copyState: "idle",
    errorMessage: null,
    isLoading: false,
    onCopy,
    onNextPage: vi.fn(),
    onOpenChange: setOpen,
    onPreviousPage: vi.fn(),
    onRetry: vi.fn(),
    open,
    page: {
      run_id: "run_large",
      result_ref: "sha256:large",
      offset: 0,
      bytes: 16_384,
      total_bytes: 70_000,
      data_base64: "",
      next_offset: 16_384,
      eof: false,
    },
    pageText: "first bounded page",
  };
  return (
    <TaskResultSection
      emptyMessage="No result recorded."
      external={controller}
      result={null}
      resultBytes={70_000}
      resultRef="sha256:large"
    />
  );
}

describe("TaskResultSection", () => {
  it("Should keep an external page closed until requested and expose bounded controls", () => {
    const onCopy = vi.fn<() => Promise<void>>().mockResolvedValue();
    render(<ExternalResultFixture onCopy={onCopy} />);

    expect(screen.getByText("70,000 bytes")).toBeInTheDocument();
    expect(screen.queryByText("first bounded page")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "View result" }));

    expect(screen.getByText("first bounded page")).toBeInTheDocument();
    expect(screen.getByText("Bytes 1–16,384 of 70,000")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous result page" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Next result page" })).toBeEnabled();

    fireEvent.click(screen.getByRole("button", { name: "Copy result" }));
    expect(onCopy).toHaveBeenCalledTimes(1);
  });
});
