import { describe, expect, it } from "vitest";

import { TONE_CLASSES, toneBg, toneSolid, toneText, toneTint } from "../tone";

describe("tone", () => {
  it("Should read accent text as brand-strong on the dark ramp (B8)", () => {
    expect(toneText("brand")).toBe("text-brand-strong");
  });

  it("Should read neutral text as neutral-ink, never muted or subtle (B8)", () => {
    expect(toneText("neutral")).toBe("text-neutral-ink");
  });

  it("Should expose bg, tint, and solid facets per tone", () => {
    expect(toneBg("success")).toBe("bg-success");
    expect(toneTint("destructive")).toBe("bg-destructive-tint");
    expect(toneSolid("brand")).toBe("bg-brand text-brand-ink");
  });

  it("Should define all six signal tones", () => {
    expect(Object.keys(TONE_CLASSES).sort()).toEqual(
      ["brand", "destructive", "info", "neutral", "success", "warning"].sort()
    );
  });
});
