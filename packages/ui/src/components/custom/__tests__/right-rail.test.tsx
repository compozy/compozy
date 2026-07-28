import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RightRail } from "../right-rail";

describe("RightRail", () => {
  it("Should render nothing when closed", () => {
    const { container } = render(
      <RightRail open={false} mode="thread">
        <span>contents</span>
      </RightRail>
    );
    expect(container.firstChild).toBeNull();
  });

  it("Should expose a labelled aside when open", () => {
    render(
      <RightRail open mode="inspector">
        <span>contents</span>
      </RightRail>
    );
    expect(screen.getByLabelText("Channel inspector")).toBeInTheDocument();
    expect(screen.getByText("contents")).toBeInTheDocument();
  });
});
