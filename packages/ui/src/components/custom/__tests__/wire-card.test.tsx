import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { WireCard, WireCardBody, WireCardFoot, WireCardHead } from "../wire-card";

describe("WireCard", () => {
  it("Should render the canonical wire-card slot by default", () => {
    const { container } = render(<WireCard>body</WireCard>);
    const card = container.querySelector<HTMLElement>('[data-slot="wire-card"]');
    expect(card).not.toBeNull();
  });

  it("Should expose data-inline=true when inline is set", () => {
    const { container } = render(<WireCard inline>body</WireCard>);
    const card = container.querySelector<HTMLElement>('[data-slot="wire-card"]');
    expect(card?.getAttribute("data-inline")).toBe("true");
  });

  it("Should render head/body/foot subcomponents", () => {
    const { container } = render(
      <WireCard>
        <WireCardHead>head</WireCardHead>
        <WireCardBody>body</WireCardBody>
        <WireCardFoot>foot</WireCardFoot>
      </WireCard>
    );
    expect(container.querySelector('[data-slot="wire-card-head"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="wire-card-body"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="wire-card-foot"]')).not.toBeNull();
  });
});
