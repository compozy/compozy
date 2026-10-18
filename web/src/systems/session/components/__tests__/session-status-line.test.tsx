// Suite: Session status line wrapper contract
// Invariant: SessionStatusLine forwards intrinsic span props while retaining its daemon-derived
// content, and a continued/forked session shows its origin as a pill that opens the source only
// while the source is readable; other lineage kinds show nothing.
// Boundary IN: SessionStatusLine public component props and the origin view model.
// Boundary OUT: Topbar placement and daemon session-state derivation.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { continuedSessionFixture } from "../../mocks/derive-fixtures";
import { primarySessionFixture } from "../../mocks/fixtures";
import { sessionOriginView } from "../../lib/session-origin";
import type { SessionPayload } from "../../types";
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

  it("Should link a continued session to its source by the source's agent", () => {
    const child = continuedSessionFixture();
    const onOpenSource = vi.fn();
    render(
      <SessionStatusLine
        onOpenOriginSource={onOpenSource}
        origin={sessionOriginView(child, { title: "Refactor flaky manager tests" })}
        session={child}
      />
    );

    const pill = screen.getByTestId("session-origin-pill");
    expect(pill).toHaveTextContent("Continued from claude");
    expect(pill).toHaveAttribute("data-link", "true");
    fireEvent.click(pill);
    expect(onOpenSource).toHaveBeenCalledExactlyOnceWith(child.lineage?.parent_session_id);
  });

  it("Should keep the words but not the link once the source is gone", () => {
    const child = continuedSessionFixture();
    const onOpenSource = vi.fn();
    render(
      <SessionStatusLine
        onOpenOriginSource={onOpenSource}
        origin={sessionOriginView(child, null)}
        session={child}
      />
    );

    const pill = screen.getByTestId("session-origin-pill");
    expect(pill).toHaveTextContent("Continued from claude");
    expect(pill).toHaveAttribute("data-link", "false");
    expect(pill.tagName).toBe("SPAN");
    fireEvent.click(pill);
    expect(onOpenSource).not.toHaveBeenCalled();
  });

  it.each(["spawn", "provenance", "recovery"] as const)(
    "Should show no origin for a %s child",
    kind => {
      const child: SessionPayload = {
        ...continuedSessionFixture(),
        lineage: { ...continuedSessionFixture().lineage!, kind },
      };
      expect(sessionOriginView(child, { title: "Source" })).toBeNull();
    }
  );
});
