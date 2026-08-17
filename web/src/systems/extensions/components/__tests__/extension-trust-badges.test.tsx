// Suite: Extension badges
// Invariant: Each badge names one distinct daemon fact and is absent when that fact does not hold.
// Boundary IN: ExtensionFormatBadge and ExtensionTrustBadges rendering from facts already resolved.
// Boundary OUT: Fact derivation from a payload, owned by lib/extension-trust-facts; badge placement
// inside marketplace surfaces, owned by the marketplace component suites.
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ExtensionFormatBadge, ExtensionTrustBadges } from "../extension-trust-badges";
import { extensionTrustFacts } from "../../lib/extension-trust-facts";

describe("ExtensionFormatBadge", () => {
  // UT-048: the badge marks the exception, so a native extension must carry no format signal.
  it("Should name the portable format only when the daemon ingested one", () => {
    const view = render(<ExtensionFormatBadge format="agent-plugin" />);
    expect(screen.getByTestId("extension-format-badge")).toHaveTextContent("agent plugin");

    view.rerender(<ExtensionFormatBadge format="compozy" />);
    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();

    view.rerender(<ExtensionFormatBadge format={undefined} />);
    expect(screen.queryByTestId("extension-format-badge")).not.toBeInTheDocument();
  });

  it("Should stay a distinct label instead of merging into the trust badges", () => {
    const facts = extensionTrustFacts({
      digest_matched: true,
      installed_from: "marketplace_registry",
      registry_tier: "community",
    });

    render(
      <>
        <ExtensionFormatBadge format="agent-plugin" />
        <ExtensionTrustBadges facts={facts} />
      </>
    );

    // Format identity and archive integrity are separate facts: neither absorbs the other's copy.
    expect(screen.getByTestId("extension-format-badge")).toHaveTextContent("agent plugin");
    expect(screen.getByTestId("extension-digest-matched-badge")).toHaveTextContent(
      "Digest matched"
    );
    expect(screen.getByTestId("extension-registry-tier-badge")).toHaveTextContent("community");
  });
});
