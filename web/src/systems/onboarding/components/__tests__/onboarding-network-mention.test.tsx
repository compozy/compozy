import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";

import { OnboardingNetworkMention } from "../onboarding-network-mention";

describe("OnboardingNetworkMention", () => {
  it("Should defer Network navigation until onboarding is complete", () => {
    render(<OnboardingNetworkMention />);
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByTestId("onboarding-network-mention")).toHaveTextContent(
      /Explore Network after setup/i
    );
  });
});
