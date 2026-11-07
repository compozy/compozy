import { describe, expect, it } from "vitest";

import { byteRangesToCharRanges, highlightSegments } from "../byte-ranges";
import { findInText, firstMatchFrom } from "../find-in-text";
import { parseGoToFileQuery } from "../go-to-file-query";
import { SEARCH_EXCERPT_CHARS, searchExcerpt } from "../search-excerpt";

// Suite: explorer search text model (file-explorer US-013–US-015; UT-093).
// Invariant: daemon byte ranges land on the right UTF-16 characters and stay inside the
// excerpt; Find in file finds every literal match by line. Owning layer: files `lib` (pure).
describe("search text model", () => {
  it("Should convert UTF-8 byte ranges to character ranges", () => {
    // "é" is 2 bytes, "😀" is 4 bytes and 2 UTF-16 units.
    const text = "é😀foo";
    expect(byteRangesToCharRanges(text, [[6, 9]])).toEqual([[3, 6]]);
    expect(byteRangesToCharRanges(text, [[0, 2]])).toEqual([[0, 1]]);
    expect(highlightSegments("a foo b", [[2, 5]])).toEqual([
      { text: "a ", match: false },
      { text: "foo", match: true },
      { text: " b", match: false },
    ]);
  });

  // UT-093
  it("Should clamp ranges inside a minified line's excerpt and mark both ellipses", () => {
    const text = `${"x".repeat(150)}needle${"y".repeat(SEARCH_EXCERPT_CHARS - 156)}`;
    const excerpt = searchExcerpt({
      line: 1,
      column: 2351,
      text,
      text_offset: 2200,
      ranges: [
        [150, 156],
        [290, 420],
      ],
    });
    expect([...excerpt.text].length).toBeLessThanOrEqual(SEARCH_EXCERPT_CHARS);
    expect(excerpt.leading).toBe(true);
    expect(excerpt.trailing).toBe(true);
    expect(excerpt.ranges).toEqual([
      [150, 156],
      [290, 300],
    ]);
    expect(excerpt.text.slice(150, 156)).toBe("needle");
  });

  it("Should drop indentation without shifting the highlighted text", () => {
    const excerpt = searchExcerpt({
      line: 41,
      column: 19,
      text: "  const changes = useSessionChanges(sessionId);",
      text_offset: 0,
      ranges: [[18, 35]],
    });
    expect(excerpt.leading).toBe(false);
    expect(excerpt.text.slice(...(excerpt.ranges[0] ?? [0, 0]))).toBe("useSessionChanges");
  });

  it("Should find literal matches per line and start from the opened line", () => {
    const matches = findInText("foo\nbar FOO foo\n", "foo", false);
    expect(matches).toEqual([
      { line: 1, start: 0, end: 3 },
      { line: 2, start: 4, end: 7 },
      { line: 2, start: 8, end: 11 },
    ]);
    expect(findInText("a.b axb", "a.b", true)).toEqual([{ line: 1, start: 0, end: 3 }]);
    expect(firstMatchFrom(matches, 2)).toBe(1);
    expect(firstMatchFrom(matches, 9)).toBe(0);
  });

  it("Should split a Go to file line suffix", () => {
    expect(parseGoToFileQuery("main.go:42")).toEqual({ name: "main.go", line: 42 });
    expect(parseGoToFileQuery("main.go:42:7")).toEqual({ name: "main.go", line: 42 });
    expect(parseGoToFileQuery(" sesrevpan ")).toEqual({ name: "sesrevpan" });
  });
});
