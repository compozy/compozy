import { describe, expect, it } from "vitest";

import {
  computeStableSessionRows,
  deriveSessionRows,
  EMPTY_STABLE_SESSION_ROWS,
  visibleWorkEntries,
  type SessionTimelinePart,
  type SessionTimelineToolPart,
} from "../session-timeline.logic";

function tool(
  index: number,
  overrides: Partial<SessionTimelineToolPart> = {}
): SessionTimelineToolPart {
  return {
    kind: "tool",
    id: `tool-${index}`,
    toolCallId: `tool-call-${index}`,
    toolName: "Read",
    args: { file_path: `/tmp/file-${index}.ts` },
    result: { content: `file-${index}` },
    status: "settled",
    turnId: "turn-1",
    timestamp: `2026-07-07T12:00:0${Math.min(index, 9)}Z`,
    ...overrides,
  };
}

function text(
  id: string,
  value: string,
  turnId?: string,
  timestamp = "2026-07-07T12:00:09Z"
): SessionTimelinePart {
  return { kind: "text", id, text: value, turnId, timestamp, state: "done" };
}

describe("session timeline derivation", () => {
  it("Should fold 8 settled consecutive tool parts into one work cluster with a +7 toggle", () => {
    const rows = deriveSessionRows(Array.from({ length: 8 }, (_, index) => tool(index + 1)));

    expect(rows).toHaveLength(2);
    const [workRow, toggleRow] = rows;
    if (workRow?.kind !== "work") throw new Error("expected work row");
    expect(workRow.entries).toHaveLength(8);
    expect(workRow.grouped).toBe(true);
    expect(workRow.visibleCount).toBe(1);
    expect(visibleWorkEntries(workRow).map(entry => entry.id)).toEqual(["tool-8"]);
    expect(toggleRow).toMatchObject({ kind: "work-toggle", hiddenCount: 7, expanded: false });
  });

  it("Should reveal every entry once the work group is marked expanded", () => {
    const groupId = "work:turn-1:tool-1";
    const rows = deriveSessionRows(
      Array.from({ length: 8 }, (_, index) => tool(index + 1)),
      { expandedWorkGroupIds: new Set([groupId]) }
    );

    const workRow = rows[0];
    if (workRow?.kind !== "work") throw new Error("expected work row");
    expect(workRow.groupId).toBe(groupId);
    expect(workRow.expanded).toBe(true);
    expect(visibleWorkEntries(workRow)).toHaveLength(8);
    expect(rows[1]).toMatchObject({ kind: "work-toggle", expanded: true });
  });

  it("Should break the cluster into two groups when text and reasoning interleave", () => {
    const parts: SessionTimelinePart[] = [
      tool(1),
      tool(2),
      text("text-break", "Narration"),
      { kind: "reasoning", id: "reason-break", text: "Thinking", turnId: "turn-1", state: "done" },
      tool(3),
    ];

    const rows = deriveSessionRows(parts);

    expect(rows.map(row => row.kind)).toEqual(["work", "work-toggle", "text", "reasoning", "work"]);
  });

  it("Should cap active work at four visible calls while keeping the group id stable while streaming", () => {
    const four = deriveSessionRows(
      Array.from({ length: 4 }, (_, index) =>
        tool(index + 1, { status: "running", result: undefined })
      )
    );
    const five = deriveSessionRows(
      Array.from({ length: 5 }, (_, index) =>
        tool(index + 1, { status: "running", result: undefined })
      )
    );

    // Four running tools stay inline (no overflow), and a fifth streamed part
    // keeps the same group id, capping the visible set at the four latest.
    expect(four).toHaveLength(1);
    expect(four[0]?.id).toBe(five[0]?.id);
    const grouped = five[0];
    if (grouped?.kind !== "work") throw new Error("expected work row");
    expect(grouped.visibleCount).toBe(4);
    expect(visibleWorkEntries(grouped).map(entry => entry.id)).toEqual([
      "tool-2",
      "tool-3",
      "tool-4",
      "tool-5",
    ]);
    expect(five[1]).toMatchObject({ kind: "work-toggle", hiddenCount: 1 });
  });

  it("Should keep unchanged row references across passes and leave prior rows intact on append", () => {
    const first = deriveSessionRows([text("stable-text", "Stable"), tool(1)]);
    const firstStable = computeStableSessionRows(first, EMPTY_STABLE_SESSION_ROWS);

    // Re-parse with fresh part objects of identical content (simulates the hook's
    // per-message re-parse): unchanged rows must keep referential identity.
    const second = deriveSessionRows([text("stable-text", "Stable"), tool(1)]);
    const secondStable = computeStableSessionRows(second, firstStable);
    expect(secondStable).toBe(firstStable);
    expect(secondStable.result[0]).toBe(firstStable.result[0]);
    expect(secondStable.result[1]).toBe(firstStable.result[1]);

    const third = deriveSessionRows([
      text("stable-text", "Stable"),
      tool(1),
      text("appended", "New", "turn-1", "2026-07-07T12:00:12Z"),
    ]);
    const thirdStable = computeStableSessionRows(third, secondStable);
    expect(thirdStable).not.toBe(secondStable);
    expect(thirdStable.result[0]).toBe(secondStable.result[0]);
    expect(thirdStable.result[1]).toBe(secondStable.result[1]);
    expect(thirdStable.result[2]?.kind).toBe("text");
  });

  it("Should give only the mutated row a new reference while sibling rows keep identity", () => {
    const first = deriveSessionRows([text("stable-a", "Alpha"), text("mutating-b", "Bravo")]);
    const firstStable = computeStableSessionRows(first, EMPTY_STABLE_SESSION_ROWS);

    // Re-derive with fresh part objects where only the second row's visible text
    // changed (simulates a streaming chunk landing on the live row).
    const second = deriveSessionRows([
      text("stable-a", "Alpha"),
      text("mutating-b", "Bravo, revised"),
    ]);
    const secondStable = computeStableSessionRows(second, firstStable);

    expect(secondStable).not.toBe(firstStable);
    // The unchanged sibling keeps its reference; only the mutated row is fresh.
    expect(secondStable.result[0]).toBe(firstStable.result[0]);
    expect(secondStable.result[1]).not.toBe(firstStable.result[1]);
    expect(secondStable.result[1]).toBe(second[1]);
  });

  it("Should treat the derivation as a pure view that never mutates the message parts", () => {
    const parts = Object.freeze([tool(1), tool(2), tool(3)]) as readonly SessionTimelinePart[];
    const snapshot = parts.map(part => ({ ...part }));

    expect(() =>
      deriveSessionRows(parts, { foldSettledTurns: true, expandedWorkGroupIds: new Set() })
    ).not.toThrow();

    expect(parts).toHaveLength(3);
    parts.forEach((part, index) => {
      expect(part).toEqual(snapshot[index]);
    });
  });

  it("Should return the previous stable state when a re-derivation reorders rows to a new result", () => {
    const rows = deriveSessionRows([
      text("first", "First", undefined, "2026-07-07T12:00:00Z"),
      text("second", "Second", undefined, "2026-07-07T12:00:10Z"),
    ]);
    const initial = computeStableSessionRows(rows, EMPTY_STABLE_SESSION_ROWS);

    const reordered = computeStableSessionRows([rows[1]!, rows[0]!], initial);

    expect(reordered).not.toBe(initial);
    expect(reordered.result[0]).toBe(initial.result[1]);
    expect(reordered.result[1]).toBe(initial.result[0]);
  });

  it("Should fold settled turns while leaving the terminal assistant text visible", () => {
    const rows = deriveSessionRows(
      [
        {
          kind: "reasoning",
          id: "reason-1",
          text: "Need context",
          turnId: "turn-fold",
          timestamp: "2026-07-07T12:00:00Z",
          state: "done",
        },
        tool(1, { turnId: "turn-fold", timestamp: "2026-07-07T12:00:03Z" }),
        text("terminal", "Done", "turn-fold", "2026-07-07T12:00:05Z"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "text"]);
    const foldRow = rows[0];
    if (foldRow?.kind !== "turn-fold") throw new Error("expected fold row");
    // Duration is the span between the turn's first and last fixture timestamps.
    expect(foldRow.label).toBe("Worked for 5s");
    expect(foldRow.durationMs).toBe(5000);
    expect(foldRow.interrupted).toBe(false);
    expect(foldRow.rows.map(row => row.kind)).toEqual(["reasoning", "work"]);
    // The terminal assistant message is never folded inside the disclosure.
    expect(foldRow.rows.some(row => row.kind === "text")).toBe(false);
    expect(rows[1]).toMatchObject({ kind: "text", id: "text:terminal" });
  });

  it("Should keep permission decisions outside a settled turn fold", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { turnId: "turn-permission", timestamp: "2026-07-07T12:00:00Z" }),
        {
          kind: "data",
          id: "permission-rejected",
          name: "data-agh-permission",
          data: {
            type: "permission",
            request_id: "turn-permission:permission-rejected",
            decision: "reject-always",
          },
          turnId: "turn-permission",
          timestamp: "2026-07-07T12:00:04Z",
          state: "done",
        },
        text("terminal-permission", "Rejected", "turn-permission", "2026-07-07T12:00:05Z"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "data", "text"]);
    const [foldRow, permissionRow] = rows;
    if (foldRow?.kind !== "turn-fold" || permissionRow?.kind !== "data") {
      throw new Error("expected a fold followed by a persistent permission row");
    }
    expect(foldRow.rows.map(row => row.kind)).toEqual(["work"]);
    expect(permissionRow.part.name).toBe("data-agh-permission");
  });

  it("Should keep the live turn inline instead of folding it", () => {
    const rows = deriveSessionRows(
      [
        tool(1, {
          status: "running",
          result: undefined,
          turnId: "turn-live",
          timestamp: "2026-07-07T12:00:00Z",
        }),
        text("terminal-live", "Still working", "turn-live", "2026-07-07T12:00:03Z"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.some(row => row.kind === "turn-fold")).toBe(false);
    expect(rows.map(row => row.kind)).toEqual(["work", "text"]);
  });

  it("Should fold an interrupted turn expanded and label the interruption", () => {
    const rows = deriveSessionRows(
      [
        {
          kind: "reasoning",
          id: "reason-int",
          text: "Working",
          turnId: "turn-int",
          timestamp: "2026-07-07T12:00:00Z",
          state: "done",
        },
        tool(1, {
          turnId: "turn-int",
          timestamp: "2026-07-07T12:00:04Z",
          status: "interrupted",
          state: "interrupted",
        }),
        text("terminal-int", "Stopped early", "turn-int", "2026-07-07T12:00:07Z"),
      ],
      { foldSettledTurns: true }
    );

    // An interrupted turn still derives a turn-fold row, but the render keeps it
    // expanded; the label swaps to the "You stopped" language and the terminal
    // assistant message stays visible below.
    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "text"]);
    const foldRow = rows[0];
    if (foldRow?.kind !== "turn-fold") throw new Error("expected fold row");
    expect(foldRow.interrupted).toBe(true);
    expect(foldRow.label).toBe("You stopped after 7s");
    expect(foldRow.rows.map(row => row.kind)).toEqual(["reasoning", "work"]);
    expect(rows[1]).toMatchObject({ kind: "text", id: "text:terminal-int" });
  });

  it("Should label a turn interrupted via the interruptedTurnIds option", () => {
    const rows = deriveSessionRows(
      [
        {
          kind: "reasoning",
          id: "reason-opt",
          text: "Working",
          turnId: "turn-opt",
          timestamp: "2026-07-07T12:00:00Z",
          state: "done",
        },
        tool(1, { turnId: "turn-opt", timestamp: "2026-07-07T12:00:05Z" }),
        text("terminal-opt", "Stopped", "turn-opt", "2026-07-07T12:00:05Z"),
      ],
      { foldSettledTurns: true, interruptedTurnIds: new Set(["turn-opt"]) }
    );

    const foldRow = rows[0];
    if (foldRow?.kind !== "turn-fold") throw new Error("expected fold row");
    expect(foldRow.interrupted).toBe(true);
    expect(foldRow.label).toBe("You stopped after 5s");
  });

  it("Should fold a settled turn behind a plain Worked label when the duration is unknown", () => {
    const rows = deriveSessionRows(
      [
        { kind: "reasoning", id: "reason-nd", text: "Thinking", turnId: "turn-nd", state: "done" },
        { kind: "text", id: "text-nd", text: "Done", turnId: "turn-nd", state: "done" },
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "text"]);
    const foldRow = rows[0];
    if (foldRow?.kind !== "turn-fold") throw new Error("expected fold row");
    expect(foldRow.interrupted).toBe(false);
    expect(foldRow.durationMs).toBe(0);
    expect(foldRow.label).toBe("Worked");
  });

  it("Should group consecutive reasoning parts into one row carrying the update count", () => {
    const parts: SessionTimelinePart[] = [
      {
        kind: "reasoning",
        id: "reason-1",
        text: "First thought",
        turnId: "turn-1",
        timestamp: "2026-07-07T12:00:00Z",
        state: "done",
      },
      {
        kind: "reasoning",
        id: "reason-2",
        text: "Second thought",
        turnId: "turn-1",
        timestamp: "2026-07-07T12:00:01Z",
        state: "done",
      },
      {
        kind: "reasoning",
        id: "reason-3",
        text: "Third thought",
        turnId: "turn-1",
        timestamp: "2026-07-07T12:00:02Z",
        state: "done",
      },
    ];

    const rows = deriveSessionRows(parts);

    expect(rows).toHaveLength(1);
    const row = rows[0];
    if (row?.kind !== "reasoning") throw new Error("expected reasoning row");
    // The three consecutive parts collapse into one "3 updates" row anchored to
    // the first part's id, with every part's text preserved in order.
    expect(row.updateCount).toBe(3);
    expect(row.parts).toHaveLength(3);
    expect(row.id).toBe("reasoning:reason-1");
    expect(row.text).toBe("First thought\n\nSecond thought\n\nThird thought");
    expect(row.streaming).toBe(false);
  });

  it("Should split reasoning groups across turn boundaries and track streaming state", () => {
    const rows = deriveSessionRows([
      { kind: "reasoning", id: "reason-a", text: "Turn one", turnId: "turn-1", state: "done" },
      { kind: "reasoning", id: "reason-b", text: "Turn two", turnId: "turn-2", state: "running" },
    ]);

    expect(rows.map(row => row.kind)).toEqual(["reasoning", "reasoning"]);
    const [first, second] = rows;
    if (first?.kind !== "reasoning" || second?.kind !== "reasoning") {
      throw new Error("expected two reasoning rows");
    }
    expect(first.updateCount).toBe(1);
    expect(first.streaming).toBe(false);
    expect(second.updateCount).toBe(1);
    // A still-streaming reasoning part keeps the row live so the turn stays open.
    expect(second.streaming).toBe(true);
  });

  it("Should keep a grouped reasoning row referentially stable across an unchanged re-derive", () => {
    const parts: SessionTimelinePart[] = [
      { kind: "reasoning", id: "reason-x", text: "One", turnId: "turn-1", state: "done" },
      { kind: "reasoning", id: "reason-y", text: "Two", turnId: "turn-1", state: "done" },
    ];

    const first = computeStableSessionRows(deriveSessionRows(parts), EMPTY_STABLE_SESSION_ROWS);
    const second = computeStableSessionRows(deriveSessionRows(parts), first);

    expect(second).toBe(first);
    expect(second.result[0]).toBe(first.result[0]);
  });
});

describe("changed-files roll-up derivation", () => {
  function editTool(
    index: number,
    filePath: string,
    oldString: string,
    newString: string
  ): SessionTimelineToolPart {
    return tool(index, {
      toolName: "Edit",
      turnId: "turn-1",
      args: { file_path: filePath, old_string: oldString, new_string: newString },
    });
  }

  it("Should aggregate a settled turn's Edit/Write files, summing repeat edits to one entry", () => {
    // File a is edited twice (+1 then +1); file b replaces three lines with one
    // (-2). Same path collapses to one first-touch-ordered entry with summed stats.
    const rows = deriveSessionRows(
      [
        editTool(1, "/src/a.ts", "a\nb", "a\nb\nc"),
        editTool(2, "/src/b.ts", "x\ny\nz", "x"),
        editTool(3, "/src/a.ts", "c", "c\nd"),
      ],
      { foldSettledTurns: true }
    );

    const rollup = rows.find(row => row.kind === "changed-files");
    if (rollup?.kind !== "changed-files") throw new Error("expected a changed-files row");
    expect(rollup.files.map(file => file.path)).toEqual(["/src/a.ts", "/src/b.ts"]);
    expect(rollup.files[0]).toMatchObject({ additions: 2, deletions: 0 });
    expect(rollup.files[1]).toMatchObject({ additions: 0, deletions: 2 });
    expect(rollup.additions).toBe(2);
    expect(rollup.deletions).toBe(2);
    expect(rollup.expanded).toBe(false);
  });

  it("Should count a Write as its full content line count with no deletions", () => {
    const rows = deriveSessionRows(
      [
        tool(1, {
          toolName: "Write",
          turnId: "turn-1",
          args: { file_path: "/src/new.ts", content: "one\ntwo\nthree" },
        }),
        text("t1", "Wrote the file.", "turn-1"),
      ],
      { foldSettledTurns: true }
    );

    const rollup = rows.find(row => row.kind === "changed-files");
    if (rollup?.kind !== "changed-files") throw new Error("expected a changed-files row");
    expect(rollup.files).toEqual([{ path: "/src/new.ts", additions: 3, deletions: 0 }]);
  });

  it("Should derive no roll-up row for a settled turn that modified no files", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { toolName: "Read", turnId: "turn-1", args: { file_path: "/src/a.ts" } }),
        tool(2, { toolName: "Bash", turnId: "turn-1", args: { command: "ls" } }),
        text("t1", "Done reviewing.", "turn-1"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.some(row => row.kind === "changed-files")).toBe(false);
  });

  it("Should not derive a roll-up while the editing turn is still active (display-only settled)", () => {
    const rows = deriveSessionRows([editTool(1, "/src/a.ts", "a", "a\nb")], {
      foldSettledTurns: true,
      activeTurnId: "turn-1",
    });

    expect(rows.some(row => row.kind === "changed-files")).toBe(false);
  });

  it("Should exclude failed edits from the roll-up", () => {
    const rows = deriveSessionRows(
      [
        tool(1, {
          toolName: "Edit",
          turnId: "turn-1",
          args: { file_path: "/src/broken.ts", old_string: "a", new_string: "b" },
          isError: true,
          result: undefined,
        }),
        text("t1", "The edit failed.", "turn-1"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.some(row => row.kind === "changed-files")).toBe(false);
  });

  it("Should keep the changed-files row referentially stable across an unchanged re-derive", () => {
    const parts: SessionTimelinePart[] = [
      tool(1, {
        toolName: "Write",
        turnId: "turn-1",
        args: { file_path: "/src/a.ts", content: "a\nb\nc" },
      }),
      text("t1", "Wrote the file.", "turn-1"),
    ];
    const options = { foldSettledTurns: true };

    const first = computeStableSessionRows(
      deriveSessionRows(parts, options),
      EMPTY_STABLE_SESSION_ROWS
    );
    const second = computeStableSessionRows(deriveSessionRows(parts, options), first);

    expect(second).toBe(first);
    expect(second.result.some(row => row.kind === "changed-files")).toBe(true);
  });
});
