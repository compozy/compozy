import { describe, expect, it } from "vitest";

import { TONE_CLASSES, toneBg, toneSolid, toneText, toneTint } from "../tone";

describe("tone", () => {
  it("Should read accent text as accent-strong on the dark ramp (B8)", () => {
    expect(toneText("accent")).toBe("text-accent-strong");
  });

  it("Should read neutral text as neutral-ink, never muted or subtle (B8)", () => {
    expect(toneText("neutral")).toBe("text-neutral-ink");
  });

  it("Should expose bg, tint, and solid facets per tone", () => {
    expect(toneBg("success")).toBe("bg-success");
    expect(toneTint("danger")).toBe("bg-danger-tint");
    expect(toneSolid("accent")).toBe("bg-accent text-accent-ink");
  });

  it("Should define all six signal tones", () => {
    expect(Object.keys(TONE_CLASSES).sort()).toEqual(
      ["accent", "danger", "info", "neutral", "success", "warning"].sort()
    );
  });
});
