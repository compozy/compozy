import { describe, expect, it } from "vitest";

import { providerLogoKey } from "../provider-logo-key";

describe("providerLogoKey", () => {
  it("Should show the runtime family logo for an alias such as claude-mm1", () => {
    expect(providerLogoKey({ name: "claude-mm1", settings: { runtime_provider: "claude" } })).toBe(
      "claude"
    );
  });

  it("Should fall back to the provider name when no runtime provider is set", () => {
    expect(providerLogoKey({ name: "Codex", settings: {} })).toBe("codex");
    expect(providerLogoKey({ name: "my-agent" })).toBe("my-agent");
  });

  it("Should keep a built-in's own brand over the vendor it runs on", () => {
    expect(providerLogoKey({ name: "pi", settings: { runtime_provider: "anthropic" } })).toBe("pi");
  });
});
