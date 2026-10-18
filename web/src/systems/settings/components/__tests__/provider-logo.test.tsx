import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ProviderLogo } from "../provider-logo";

describe("ProviderLogo", () => {
  it("normalizes provider casing before selecting the icon", () => {
    const { container } = render(<ProviderLogo provider="Qwen-Code" />);
    const logo = container.querySelector('[data-slot="provider-logo"][data-provider="qwen-code"]');
    expect(logo).toBeTruthy();
    expect(logo?.querySelector("svg")).toBeTruthy();
  });
});
