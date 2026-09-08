import { describe, expect, it } from "vitest";

import {
  formatPayloadSize,
  PAYLOAD_PREVIEW_MAX_LINES,
  payloadOverflows,
  payloadTruncationNote,
  truncatePayload,
} from "../session-payload-truncation";

// Suite: payload truncation model (US-019.EC-2, UT-106).
// Invariant: an oversized inline payload renders its head with a truthful note
// ("Showing the first N of M lines · size") and expand/download affordances;
// expanding renders everything; a payload inside the bound is untouched.
describe("payload truncation", () => {
  const lines = Array.from({ length: 12_480 }, (_, index) => `line ${index + 1}`);
  const giant = lines.join("\n");

  it("Should truncate to the preview head and describe the whole", () => {
    const model = truncatePayload(giant);
    expect(model.truncated).toBe(true);
    expect(model.shownLines).toBe(PAYLOAD_PREVIEW_MAX_LINES);
    expect(model.totalLines).toBe(12_480);
    expect(model.text.split("\n")).toHaveLength(PAYLOAD_PREVIEW_MAX_LINES);
    expect(model.text.endsWith("line 200")).toBe(true);
    expect(payloadTruncationNote(model)).toBe("Showing the first 200 of 12,480 lines");
    expect(payloadOverflows(giant)).toBe(true);
  });

  it("Should render everything once expanded and leave small payloads alone", () => {
    expect(truncatePayload(giant, { expanded: true })).toMatchObject({
      truncated: false,
      shownLines: 12_480,
      text: giant,
    });
    const small = "one\ntwo\nthree";
    expect(truncatePayload(small)).toEqual({
      text: small,
      truncated: false,
      shownLines: 3,
      totalLines: 3,
      bytes: 13,
    });
    expect(truncatePayload("")).toMatchObject({ totalLines: 0, truncated: false });
    expect(payloadOverflows(small)).toBe(false);
  });

  it("Should size in bytes with one decimal only above a megabyte", () => {
    expect(formatPayloadSize(96)).toBe("96 B");
    expect(formatPayloadSize(421_888)).toBe("412 KB");
    expect(formatPayloadSize(1_887_436)).toBe("1.8 MB");
    expect(truncatePayload("héllo").bytes).toBe(6);
  });
});
