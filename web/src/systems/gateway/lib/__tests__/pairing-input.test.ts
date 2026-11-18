// Suite: pairing redeem input
// Invariant: the redeem gate sends exactly one credential. Six digits, with or without grouping
// spaces or dashes, are a manual code; anything else is an artifact or link, exactly as before.
// Owning layer: the pure input classifier behind the gateway redeem gate.
import { describe, expect, it } from "vitest";

import { classifyPairingInput } from "../pairing-input";

describe("classifyPairingInput", () => {
  it.each(["123456", "123 456", "123-456", "  1 2 3-4 5 6  "])(
    "Should classify %j as the manual code 123456 with no other credential",
    input => {
      expect(classifyPairingInput(input)).toStrictEqual({ manualCode: "123456" });
    }
  );

  it.each([
    "cpz_gwp_2f8c1d5a9b4e7c0d",
    "1234567",
    "12345",
    "123 4567",
    "https://example.test/not-a-pairing-link",
  ])("Should keep %j as an artifact with no manual code", input => {
    const credential = classifyPairingInput(input);
    expect(credential).toStrictEqual({ artifact: input.trim() });
  });

  it("Should extract the artifact from a pasted pairing link fragment", () => {
    expect(
      classifyPairingInput("https://private.gateway.test/#pair=cpz_gwp_2f8c1d5a9b4e7c0d")
    ).toStrictEqual({ artifact: "cpz_gwp_2f8c1d5a9b4e7c0d" });
  });

  it("Should extract the artifact from a pasted pairing link query", () => {
    expect(
      classifyPairingInput("https://private.gateway.test/?pair=cpz_gwp_2f8c1d5a9b4e7c0d")
    ).toStrictEqual({ artifact: "cpz_gwp_2f8c1d5a9b4e7c0d" });
  });

  it("Should treat blank input as nothing to redeem", () => {
    expect(classifyPairingInput("   ")).toBeNull();
    expect(classifyPairingInput("")).toBeNull();
  });
});
