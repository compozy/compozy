// Suite: Session status line wrapper contract
// Invariant: SessionStatusLine forwards intrinsic span props while retaining its daemon-derived content.
// Boundary IN: SessionStatusLine public component props.
// Boundary OUT: Topbar placement and daemon session-state derivation.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../mocks/fixtures";
import { SessionStatusLine } from "../session-status-line";

describe("SessionStatusLine", () => {
  it("Should forward native span props", () => {
    const onClick = vi.fn();
    render(
      <SessionStatusLine
        aria-label="Current session identity"
        className="consumer-status-line"
        data-testid="custom-session-status"
        onClick={onClick}
        session={primarySessionFixture}
      />
    );

    const status = screen.getByTestId("custom-session-status");
    expect(status).toHaveAttribute("aria-label", "Current session identity");
    expect(status).toHaveClass("consumer-status-line");
    fireEvent.click(status);
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
