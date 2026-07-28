import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ProviderLogo } from "../provider-logo";

describe("ProviderLogo", () => {
  it("renders fallback icons for newly supported ACP providers", () => {
    const providers = [
      "blackbox",
      "cline",
      "goose",
      "hermes",
      "junie",
      "kimi-cli",
      "openclaw",
      "openhands",
      "qoder",
      "qwen-code",
    ];

    for (const provider of providers) {
      const { container, unmount } = render(<ProviderLogo provider={provider} />);
      const logo = container.querySelector(
        `[data-slot="provider-logo"][data-provider="${provider}"]`
      );
      expect(logo).toBeTruthy();
      expect(logo?.querySelector("svg")).toBeTruthy();
      unmount();
    }
  });

  it("normalizes provider casing before selecting the icon", () => {
    const { container } = render(<ProviderLogo provider="Qwen-Code" />);
    const logo = container.querySelector('[data-slot="provider-logo"][data-provider="qwen-code"]');
    expect(logo).toBeTruthy();
    expect(logo?.querySelector("svg")).toBeTruthy();
  });
});
