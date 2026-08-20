import { describe, expect, it } from "vitest";

import {
  PRODUCT_METHODS,
  validProductEventPayload,
  validProductParams,
  validProductResponse,
} from "../product-contract";

describe("product preload contract", () => {
  it("Should allow only the declared product methods", () => {
    expect(PRODUCT_METHODS.has("global_shortcuts.sync")).toBe(true);
    expect(PRODUCT_METHODS.has("global_shortcuts.status")).toBe(true);
    expect(PRODUCT_METHODS.has("shell.execute")).toBe(false);
  });

  it("Should validate sync parameters strictly", () => {
    expect(
      validProductParams("global_shortcuts.sync", {
        bindings: [{ command_id: "palette.summon.global", chord: "meta+shift+Space" }],
      })
    ).toBe(true);
    expect(
      validProductParams("global_shortcuts.sync", {
        bindings: [{ command_id: "palette.summon.global", chord: "meta+shift+Space", extra: true }],
      })
    ).toBe(false);
    expect(validProductParams("global_shortcuts.status", { extra: true })).toBe(false);
  });

  it("Should validate registration responses and summon payloads", () => {
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          active_chord: "meta+shift+Space",
          status: "registered",
        },
      ])
    ).toBe(true);
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          status: "unknown",
        },
      ])
    ).toBe(false);
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          status: "registered",
          extra: true,
        },
      ])
    ).toBe(false);
    expect(validProductEventPayload("shell:summon", { command_id: "palette.open" })).toBe(true);
    expect(
      validProductEventPayload("shell:summon", { command_id: "palette.open", extra: true })
    ).toBe(false);
  });
});
