// Suite: loopback-only truthful state (UT-008, S5)
// Invariant: when the loopback-only signal fires, the state names the machine
// running CompozyOS as where the action belongs — neutral/informative, never
// error-red — and reads from the registered COPY.md strings, not ad-hoc prose.
// Owning layer: the gateway system's domain component.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LOOPBACK_ONLY_STATE_COPY } from "../../lib/gateway-copy";
import { LoopbackOnlyState } from "../loopback-only-state";

describe("LoopbackOnlyState", () => {
  it("Should render daemon-host guidance in the panel layout", () => {
    render(<LoopbackOnlyState />);

    const state = screen.getByTestId("gateway-loopback-only-state");
    expect(state).toHaveTextContent(LOOPBACK_ONLY_STATE_COPY.panel.title);
    expect(state).toHaveTextContent(LOOPBACK_ONLY_STATE_COPY.panel.description);
    expect(state).toHaveTextContent(LOOPBACK_ONLY_STATE_COPY.panel.hint);
    // The guidance names the daemon host, never the wire codes or "daemon".
    expect(state).toHaveTextContent("machine running CompozyOS");
    expect(state).not.toHaveTextContent("daemon");
    expect(state).not.toHaveTextContent("loopback_mutation_required");
  });

  it("Should render the slim shell banner layout with the same host guidance", () => {
    render(<LoopbackOnlyState layout="banner" />);

    const banner = screen.getByTestId("gateway-loopback-only-banner");
    expect(banner).toHaveTextContent(LOOPBACK_ONLY_STATE_COPY.banner.title);
    expect(banner).toHaveTextContent(LOOPBACK_ONLY_STATE_COPY.banner.description);
    expect(banner).toHaveTextContent("machine running CompozyOS");
  });

  it("Should stay an informative signal, not an alert-severity failure", () => {
    render(<LoopbackOnlyState layout="banner" />);

    // Neutral variant on the banner: the daemon did not fail, it told the
    // truth about where the action belongs (BR-3's tone contract).
    const banner = screen.getByTestId("gateway-loopback-only-banner");
    expect(banner).toHaveAttribute("data-variant", "neutral");
  });
});
