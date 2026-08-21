// Suite: command-palette view patch interpreter
// Invariant: RFC 6902 test operations compare JSON objects by structural
// equality, so equivalent member order cannot reject a frame the daemon accepts.
// Owning layer: web/src/systems/os/lib/cmd-palette-view-patch.ts
// Canonical suite: this file.
import { describe, expect, it } from "vitest";

import { payloadFromProgramFrame, sameJSON } from "../cmd-palette-view-patch";
import type { CmdPaletteViewFrame, CmdPaletteViewPayload } from "../cmd-palette-types";

function payload(title: string): CmdPaletteViewPayload {
  return {
    view: "v1",
    chrome: { complete: true, on_search: "search" },
    sections: [{ rows: [{ id: "one", title }] }],
  };
}

function patchFrame(ops: NonNullable<CmdPaletteViewFrame["patch"]>["ops"]): CmdPaletteViewFrame {
  return {
    view_session: "vs_test",
    revision: "vr_2",
    generation: 1,
    handlers: ["search"],
    patch: { view_id: "ext.notes.browser", from: "vr_1", to: "vr_2", ops },
  };
}

describe("cmd-palette view patch", () => {
  it("Should treat RFC 6902 object tests as member-order independent", () => {
    const current = payload("Before");
    const next = payloadFromProgramFrame(
      current,
      patchFrame([
        { op: "test", path: "/chrome", value: { on_search: "search", complete: true } },
        { op: "replace", path: "/sections/0/rows/0/title", value: "After" },
      ])
    );
    expect(next.sections?.[0]?.rows[0]?.title).toBe("After");
  });

  it("Should compare nested objects and arrays without insertion order", () => {
    expect(sameJSON({ a: 1, b: { c: 2 } }, { b: { c: 2 }, a: 1 })).toBe(true);
    expect(sameJSON([1, { z: 1, y: 2 }], [1, { y: 2, z: 1 }])).toBe(true);
    expect(sameJSON({ a: 1 }, { a: 2 })).toBe(false);
  });
});
