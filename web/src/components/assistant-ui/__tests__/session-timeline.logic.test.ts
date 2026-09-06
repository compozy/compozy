import { describe, expect, it } from "vitest";

import {
  deriveSessionRows,
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
  it("Should collapse a settled run of 8 into one semantic summary row", () => {
    const rows = deriveSessionRows(Array.from({ length: 8 }, (_, index) => tool(index + 1)));

    expect(rows).toHaveLength(1);
    const workRow = rows[0];
    if (workRow?.kind !== "work") throw new Error("expected work row");
    expect(workRow.entries).toHaveLength(8);
    expect(workRow.summary?.label).toBe("Read 8 files");
    expect(workRow.expanded).toBe(false);
    expect(workRow.active).toBe(false);
  });

  it("Should mark the summary row expanded once the work group is toggled open", () => {
    const groupId = "work:turn-1:tool-call-1";
    const rows = deriveSessionRows(
      Array.from({ length: 8 }, (_, index) => tool(index + 1)),
      { expandedWorkGroupIds: new Set([groupId]) }
    );

    const workRow = rows[0];
    if (workRow?.kind !== "work") throw new Error("expected work row");
    expect(workRow.groupId).toBe(groupId);
    expect(workRow.expanded).toBe(true);
    expect(workRow.summary).not.toBeNull();
    expect(workRow.entries).toHaveLength(8);
  });

  it("Should keep a work disclosure expanded when the rendered part id is replaced", () => {
    const initialParts = [...Array.from({ length: 4 }, (_, index) => tool(index + 1)), tool(5)];
    const groupId = "work:turn-1:tool-call-1";
    const initialRows = deriveSessionRows(initialParts, {
      expandedWorkGroupIds: new Set([groupId]),
    });

    const rederivedParts = initialParts.map((part, index) => ({
      ...part,
      id: `replacement-${index + 1}`,
    }));
    const rederivedRows = deriveSessionRows(rederivedParts, {
      expandedWorkGroupIds: new Set([groupId]),
    });

    const initialWork = initialRows.find(row => row.kind === "work");
    const rederivedWork = rederivedRows.find(row => row.kind === "work");
    if (initialWork?.kind !== "work" || rederivedWork?.kind !== "work") {
      throw new Error("expected work rows");
    }
    expect(initialWork.groupId).toBe(groupId);
    expect(rederivedWork.groupId).toBe(groupId);
    expect(rederivedWork.expanded).toBe(true);
    expect(rederivedWork.entries).toHaveLength(5);
  });

  it("Should keep a work disclosure stable when calls reorder or duplicate events settle", () => {
    const initialParts = Array.from({ length: 5 }, (_, index) => tool(index + 1));
    const groupId = "work:turn-1:tool-call-1";
    const initialRows = deriveSessionRows(initialParts, {
      expandedWorkGroupIds: new Set([groupId]),
    });

    const reorderedRows = deriveSessionRows(
      [initialParts[1]!, initialParts[0]!, ...initialParts.slice(2)],
      { expandedWorkGroupIds: new Set([groupId]) }
    );
    const duplicateRemovedRows = deriveSessionRows(
      [
        initialParts[0]!,
        { ...initialParts[0]!, id: "duplicate-rendered-tool-1" },
        ...initialParts.slice(1),
      ],
      { expandedWorkGroupIds: new Set([groupId]) }
    );

    for (const rows of [initialRows, reorderedRows, duplicateRemovedRows]) {
      const workRow = rows.find(row => row.kind === "work");
      if (workRow?.kind !== "work") throw new Error("expected work row");
      expect(workRow.groupId).toBe(groupId);
      expect(workRow.expanded).toBe(true);
    }
  });

  it("Should retain a group's key when a lower-sorting call arrives and the group closes", () => {
    const initialParts = Array.from({ length: 5 }, (_, index) => tool(index + 1));
    const groupId = "work:turn-1:tool-call-1";
    const anchors = new Map([
      [
        groupId,
        {
          groupId,
          turnId: "turn-1",
          anchorToolCallId: "tool-call-1",
        },
      ],
    ]);

    const initialRows = deriveSessionRows(initialParts, {
      expandedWorkGroupIds: new Set([groupId]),
      workGroupAnchors: anchors,
    });
    const grownParts = [tool(0), ...initialParts];
    const grownRows = deriveSessionRows(grownParts, {
      expandedWorkGroupIds: new Set([groupId]),
      workGroupAnchors: anchors,
    });
    const closedRows = deriveSessionRows(grownParts, { workGroupAnchors: anchors });

    for (const rows of [initialRows, grownRows, closedRows]) {
      const workRow = rows.find(row => row.kind === "work");
      if (workRow?.kind !== "work") throw new Error("expected work row");
      expect(workRow.groupId).toBe(groupId);
    }
    const grownWork = grownRows.find(row => row.kind === "work");
    const closedWork = closedRows.find(row => row.kind === "work");
    if (grownWork?.kind !== "work" || closedWork?.kind !== "work") {
      throw new Error("expected grown and closed work rows");
    }
    expect(grownWork.expanded).toBe(true);
    expect(closedWork.expanded).toBe(false);
  });

  it("Should retain the completed group's identity while calls settle, failures included", () => {
    const liveParts = Array.from({ length: 3 }, (_, index) =>
      tool(index + 1, { status: "running", result: undefined })
    );
    const liveRows = deriveSessionRows(liveParts, { activeTurnId: "turn-1" });
    expect(liveRows.map(row => row.kind)).toEqual(["live-tool"]);

    // Two calls settle (one failed) while the third still runs: one completed
    // group ahead of the live row, the failure counted inside it.
    const partial = liveParts.map((part, index) =>
      index === 2
        ? part
        : index === 1
          ? { ...part, status: "settled" as const, isError: true, result: { error: "boom" } }
          : { ...part, status: "settled" as const, result: { content: "file-1" } }
    );
    const partialRows = deriveSessionRows(partial, { activeTurnId: "turn-1" });
    expect(partialRows.map(row => row.kind)).toEqual(["work", "live-tool"]);
    const group = partialRows[0];
    if (group?.kind !== "work") throw new Error("expected completed group");
    expect(group.summary?.failedCount).toBe(1);
    const anchors = new Map([
      [
        group.groupId,
        { groupId: group.groupId, turnId: "turn-1", anchorToolCallId: "tool-call-1" },
      ],
    ]);

    const settledParts = liveParts.map((part, index) =>
      index === 1
        ? { ...part, status: "settled" as const, isError: true, result: { error: "boom" } }
        : { ...part, status: "settled" as const, result: { content: `file-${index + 1}` } }
    );
    const settledRows = deriveSessionRows(settledParts, { workGroupAnchors: anchors });
    expect(settledRows.map(row => row.kind)).toEqual(["work"]);
    const settled = settledRows[0];
    if (settled?.kind !== "work") throw new Error("expected settled group");
    expect(settled.groupId).toBe(group.groupId);
    expect(settled.entries).toHaveLength(3);
    expect(settled.summary?.failedCount).toBe(1);
  });

  it("Should keep an expanded group's identity when a lower-sorting call settles into it", () => {
    const historicalGroupId = "work:turn-1:tool-call-4";
    const expandedRows = deriveSessionRows(
      [tool(4), tool(5), tool(6, { status: "running", result: undefined })],
      {
        activeTurnId: "turn-1",
        expandedWorkGroupIds: new Set([historicalGroupId]),
      }
    );
    const expandedWork = expandedRows.find(row => row.kind === "work");
    if (expandedWork?.kind !== "work") throw new Error("expected expanded work row");
    expect(expandedWork.groupId).toBe(historicalGroupId);
    expect(expandedWork.expanded).toBe(true);

    const observedAnchors = new Map([
      [
        historicalGroupId,
        {
          groupId: historicalGroupId,
          turnId: "turn-1",
          anchorToolCallId: "tool-call-1",
        },
      ],
    ]);
    const settledRows = deriveSessionRows(
      [tool(1), tool(4, { isError: true, result: { error: "boom" } }), tool(5)],
      { workGroupAnchors: observedAnchors, expandedWorkGroupIds: new Set([historicalGroupId]) }
    );
    const workRows = settledRows.filter(row => row.kind === "work");

    expect(workRows).toHaveLength(1);
    expect(workRows[0]?.groupId).toBe(historicalGroupId);
    expect(workRows[0]?.expanded).toBe(true);
    expect(workRows[0]?.summary?.failedCount).toBe(1);
  });

  it("Should break the cluster into two runs when text and reasoning interleave", () => {
    const parts: SessionTimelinePart[] = [
      tool(1),
      tool(2),
      text("text-break", "Narration"),
      { kind: "reasoning", id: "reason-break", text: "Thinking", turnId: "turn-1", state: "done" },
      tool(3),
    ];

    const rows = deriveSessionRows(parts);

    expect(rows.map(row => row.kind)).toEqual(["work", "text", "reasoning", "work"]);
    const [first, , , last] = rows;
    if (first?.kind !== "work" || last?.kind !== "work") throw new Error("expected work rows");
    // The settled 2-run collapses to a summary; the lone trailing call stays a
    // plain row (below the 2+ fold minimum).
    expect(first.summary?.label).toBe("Read 2 files");
    expect(last.summary).toBeNull();
    expect(last.entries).toHaveLength(1);
  });

  // UT-085 / UT-089 (ADR-006 rule 1): the live turn derives exactly one live row
  // for the calls still running — one call reads as itself, several stay one
  // honest row with a count — and progress ticks never become rows.
  it("Should derive exactly one live row that replaces itself and counts parallel calls", () => {
    const single = deriveSessionRows(
      [
        tool(1),
        tool(2),
        tool(3, {
          toolName: "Bash",
          args: { command: "go test" },
          status: "running",
          result: undefined,
        }),
      ],
      { activeTurnId: "turn-1" }
    );
    expect(single.map(row => row.kind)).toEqual(["work", "live-tool"]);
    const live = single[1];
    if (live?.kind !== "live-tool") throw new Error("expected live row");
    expect(live.id).toBe("live:turn-1");
    expect(live.entries.map(entry => entry.id)).toEqual(["tool-3"]);
    expect(live.agent).toBe(false);

    const parallel = deriveSessionRows(
      [
        tool(1, { status: "running", result: undefined }),
        tool(2, { status: "running", result: undefined }),
        tool(3, { status: "running", result: undefined }),
      ],
      { activeTurnId: "turn-1" }
    );
    expect(parallel).toHaveLength(1);
    const row = parallel[0];
    if (row?.kind !== "live-tool") throw new Error("expected live row");
    expect(row.entries).toHaveLength(3);
    expect(row.id).toBe("live:turn-1");
    expect(row.expanded).toBe(false);
    expect(
      deriveSessionRows(
        [
          tool(1, { status: "running", result: undefined }),
          tool(2, { status: "running", result: undefined }),
        ],
        { activeTurnId: "turn-1", expandedWorkGroupIds: new Set(["live:turn-1"]) }
      )[0]
    ).toMatchObject({ kind: "live-tool", expanded: true });
  });

  it("Should drop progress ticks so they never append rows", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { status: "running", result: undefined }),
        {
          kind: "data",
          id: "tick-1",
          name: "data-compozy-event",
          data: { type: "runtime_progress", runtime: { elapsed_seconds: 4, idle_seconds: 0 } },
          turnId: "turn-1",
        },
        {
          kind: "data",
          id: "tick-2",
          name: "data-compozy-event",
          data: { type: "runtime_progress", runtime: { elapsed_seconds: 9, idle_seconds: 0 } },
          turnId: "turn-1",
        },
      ],
      { activeTurnId: "turn-1" }
    );
    expect(rows.map(row => row.kind)).toEqual(["live-tool"]);
  });

  it("Should give each running child agent its own live row, never counted into the parallel row", () => {
    const rows = deriveSessionRows(
      [
        tool(1, {
          toolName: "Task",
          args: { description: "review the diff" },
          status: "running",
          result: undefined,
        }),
        tool(2, {
          toolName: "Task",
          args: { description: "scout the store" },
          status: "running",
          result: undefined,
        }),
        tool(3, {
          toolName: "Bash",
          args: { command: "go vet" },
          status: "running",
          result: undefined,
        }),
      ],
      { activeTurnId: "turn-1" }
    );
    expect(rows.map(row => row.kind)).toEqual(["live-tool", "live-tool", "live-tool"]);
    expect(rows.map(row => (row.kind === "live-tool" ? row.agent : null))).toEqual([
      true,
      true,
      false,
    ]);
    const shell = rows[2];
    if (shell?.kind !== "live-tool") throw new Error("expected live row");
    expect(shell.entries.map(entry => entry.id)).toEqual(["tool-3"]);
  });

  // UT-087: completed tools of the live turn collapse into one expandable group
  // sentence; a single completed call stays a row of its own.
  it("Should collapse the live turn's completed tools into one group sentence", () => {
    const parts: SessionTimelinePart[] = [
      tool(1, { toolName: "Bash", args: { command: "go test" } }),
      tool(2, { toolName: "Bash", args: { command: "go vet" } }),
      tool(3, { toolName: "Edit", args: { file_path: "/src/a.ts" } }),
      tool(4, { toolName: "Edit", args: { file_path: "/src/b.ts" } }),
      tool(5, {
        toolName: "Bash",
        args: { command: "go build" },
        status: "running",
        result: undefined,
      }),
    ];
    const rows = deriveSessionRows(parts, { activeTurnId: "turn-1" });
    expect(rows.map(row => row.kind)).toEqual(["work", "live-tool"]);
    const group = rows[0];
    if (group?.kind !== "work") throw new Error("expected group row");
    expect(group.active).toBe(true);
    expect(group.summary?.label).toBe("Ran 2 commands, edited 2 files");
    expect(group.summary?.failedCount).toBe(0);
    expect(group.expanded).toBe(false);

    const lone = deriveSessionRows([tool(1), tool(2, { status: "running", result: undefined })], {
      activeTurnId: "turn-1",
    });
    expect(lone.map(row => row.kind)).toEqual(["work", "live-tool"]);
    expect(lone[0]).toMatchObject({ kind: "work", summary: null, active: true });
  });

  it("Should keep the trailing run of the active turn open even when fully settled", () => {
    const rows = deriveSessionRows(
      Array.from({ length: 3 }, (_, index) => tool(index + 1)),
      { activeTurnId: "turn-1" }
    );

    expect(rows).toHaveLength(1);
    const workRow = rows[0];
    if (workRow?.kind !== "work") throw new Error("expected work row");
    // Completed tools of the live turn rest as the group sentence, still active.
    expect(workRow.summary?.label).toBe("Read 3 files");
    expect(workRow.active).toBe(true);
  });

  it("Should collapse an earlier run the moment it settles, even mid-turn", () => {
    const parts: SessionTimelinePart[] = [
      tool(1),
      tool(2),
      tool(3),
      text("narration", "Progress so far", "turn-1", "2026-07-07T12:00:04Z"),
      tool(4, { status: "running", result: undefined }),
      tool(5, { status: "running", result: undefined }),
    ];

    const rows = deriveSessionRows(parts, { activeTurnId: "turn-1" });

    expect(rows.map(row => row.kind)).toEqual(["work", "text", "live-tool"]);
    const [settled, , live] = rows;
    if (settled?.kind !== "work" || live?.kind !== "live-tool") throw new Error("expected rows");
    expect(settled.summary?.label).toBe("Read 3 files");
    expect(settled.active).toBe(false);
    expect(live.entries.map(entry => entry.id)).toEqual(["tool-4", "tool-5"]);
  });

  // UT-088 (ADR-009): a failure the agent absorbed stays inside the group as
  // information — counted in "· N failed" — never as turn-level danger.
  it("Should keep an absorbed failure inside the group with a failed count, not as an alarm", () => {
    const parts: SessionTimelinePart[] = [
      tool(1, { toolName: "Bash", args: { command: "go test" } }),
      tool(2, {
        toolName: "Bash",
        args: { command: "go test -count=3" },
        isError: true,
        result: { error: "FAIL" },
      }),
      tool(3, { toolName: "Edit", args: { file_path: "/src/a.ts" } }),
      tool(4, {
        toolName: "Bash",
        args: { command: "go test -race" },
        status: "running",
        result: undefined,
      }),
    ];

    const rows = deriveSessionRows(parts, { activeTurnId: "turn-1" });

    expect(rows.map(row => row.kind)).toEqual(["work", "live-tool"]);
    const group = rows[0];
    if (group?.kind !== "work") throw new Error("expected group row");
    expect(group.entries.map(entry => entry.id)).toEqual(["tool-1", "tool-2", "tool-3"]);
    expect(group.summary?.label).toBe("Ran 2 commands, edited 1 file");
    expect(group.summary?.failedCount).toBe(1);

    // A settled run with a failure folds the same way once the turn moves on.
    const settled = deriveSessionRows(parts.slice(0, 3));
    expect(settled).toHaveLength(1);
    expect(settled[0]).toMatchObject({ kind: "work", active: false });
  });

  it("Should order summary categories Ran, Edited, Read, Searched, agent, Used with distinct-file counts", () => {
    const parts: SessionTimelinePart[] = [
      tool(1, { toolName: "mcp__linear__list_issues", args: {} }),
      tool(2, { toolName: "Task", args: { prompt: "explore" } }),
      tool(3, { toolName: "Grep", args: { pattern: "Alert" } }),
      tool(4, { toolName: "Read", args: { file_path: "/src/a.ts" } }),
      tool(5, { toolName: "Read", args: { file_path: "/src/a.ts" } }),
      tool(6, {
        toolName: "Edit",
        args: { file_path: "/src/b.ts", old_string: "a", new_string: "b" },
      }),
      tool(7, { toolName: "Bash", args: { command: "ls" } }),
    ];

    const rows = deriveSessionRows(parts);

    const workRow = rows[0];
    if (workRow?.kind !== "work") throw new Error("expected work row");
    // Fixed presentation order regardless of call order; the two same-path
    // Reads count as one distinct file.
    expect(workRow.summary?.label).toBe(
      "Ran 1 command, edited 1 file, read 1 file, searched 1 file, ran 1 agent task, used 1 tool"
    );
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
    expect(foldRow.cause).toBe("settled");
    expect(foldRow.open).toBe(false);
    // A lone Read has no group sentence, so the fold carries the duration alone.
    expect(foldRow.counts).toBeNull();
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
          name: "data-compozy-permission",
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
    expect(permissionRow.part.name).toBe("data-compozy-permission");
  });

  it("Should keep a settled deliberate terminal block outside a turn fold and summary", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { turnId: "turn-terminal", timestamp: "2026-08-26T12:00:00Z" }),
        tool(2, { turnId: "turn-terminal", timestamp: "2026-08-26T12:00:01Z" }),
        tool(3, {
          toolName: "compozy__terminal_exec",
          args: { command: "bun run dev" },
          result: {
            rawOutput: { terminal_id: "term-4f21c9a03b7e", output: "ready", still_running: true },
          },
          turnId: "turn-terminal",
          timestamp: "2026-08-26T12:00:02Z",
        }),
        text("terminal-answer", "The server is up.", "turn-terminal", "2026-08-26T12:00:05Z"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "work", "text"]);
    const [foldRow, terminalRow] = rows;
    if (foldRow?.kind !== "turn-fold" || terminalRow?.kind !== "work") {
      throw new Error("expected a fold followed by a persistent terminal work row");
    }
    expect(foldRow.rows.map(row => row.kind)).toEqual(["work"]);
    expect(terminalRow.summary).toBeNull();
    expect(terminalRow.entries.map(entry => entry.toolName)).toEqual(["compozy__terminal_exec"]);
    const foldedWork = foldRow.rows[0];
    if (foldedWork?.kind !== "work") throw new Error("expected folded work");
    expect(foldedWork.entries.map(entry => entry.toolName)).toEqual(["Read", "Read"]);
    // Inside the fold the calls read as rows — no second disclosure; the sentence lives on the fold.
    expect(foldedWork.summary).toBeNull();
    expect(foldRow.counts).toBe("Read 2 files");
  });

  it("Should keep a settled terminal open outside a turn fold and summary", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { turnId: "turn-open", timestamp: "2026-08-26T12:00:00Z" }),
        tool(2, { turnId: "turn-open", timestamp: "2026-08-26T12:00:01Z" }),
        tool(3, {
          toolName: "compozy__terminal_open",
          args: { title: "Build logs" },
          result: { rawOutput: { terminal_id: "term-a03b558d21f0" } },
          turnId: "turn-open",
          timestamp: "2026-08-26T12:00:02Z",
        }),
        text("open-answer", "The terminal is open.", "turn-open", "2026-08-26T12:00:05Z"),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "work", "text"]);
    const [foldRow, terminalRow] = rows;
    if (foldRow?.kind !== "turn-fold" || terminalRow?.kind !== "work") {
      throw new Error("expected a fold followed by a persistent terminal work row");
    }
    expect(terminalRow.summary).toBeNull();
    expect(terminalRow.entries.map(entry => entry.toolName)).toEqual(["compozy__terminal_open"]);
    const foldedWork = foldRow.rows[0];
    if (foldedWork?.kind !== "work") throw new Error("expected folded work");
    expect(foldedWork.entries.map(entry => entry.toolName)).toEqual(["Read", "Read"]);
  });

  it("Should keep a running terminal as its own live row beside the live tail", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { turnId: "turn-live-terminal", timestamp: "2026-08-26T12:00:00Z" }),
        tool(2, { turnId: "turn-live-terminal", timestamp: "2026-08-26T12:00:01Z" }),
        tool(3, {
          toolName: "compozy__terminal_exec",
          status: "running",
          args: { command: "bun run dev" },
          turnId: "turn-live-terminal",
          timestamp: "2026-08-26T12:00:02Z",
        }),
      ],
      { activeTurnId: "turn-live-terminal" }
    );

    const work = rows.filter(row => row.kind === "work");
    expect(work).toHaveLength(2);
    expect(work[0]?.entries.map(entry => entry.toolName)).toEqual(["Read", "Read"]);
    expect(work[0]?.summary?.label).toBe("Read 2 files");
    expect(work[0]?.active).toBe(true);
    expect(work[1]?.entries.map(entry => entry.toolName)).toEqual(["compozy__terminal_exec"]);
    expect(work[1]?.summary).toBeNull();
    expect(work[1]?.active).toBe(true);
  });

  it("Should fold generic data events into a settled turn", () => {
    const rows = deriveSessionRows(
      [
        tool(1, { turnId: "turn-generic-data", timestamp: "2026-08-26T12:00:00Z" }),
        {
          kind: "data",
          id: "runtime-progress-1",
          name: "data-compozy-event",
          // A runtime warning is a visible row (a status event would render nothing and never become one).
          data: {
            type: "runtime_warning",
            text: "Provider slow to answer",
            runtime: { elapsed_seconds: 40, idle_seconds: 30 },
          },
          turnId: "turn-generic-data",
          timestamp: "2026-08-26T12:00:04Z",
          state: "done",
        },
        text(
          "terminal-reported",
          "The terminal report is complete.",
          "turn-generic-data",
          "2026-08-26T12:00:05Z"
        ),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["turn-fold", "text"]);
    const [foldRow] = rows;
    if (foldRow?.kind !== "turn-fold") {
      throw new Error("expected the settled turn to fold its data event");
    }
    expect(foldRow.rows.map(row => row.kind)).toEqual(["work", "data"]);
  });

  it("Should keep every text segment visible when a permission splits the response", () => {
    const rows = deriveSessionRows(
      [
        text(
          "before-permission",
          "Streaming response started.",
          "turn-split",
          "2026-07-07T12:00:00Z"
        ),
        {
          kind: "data",
          id: "permission-allowed",
          name: "data-compozy-permission",
          data: {
            type: "permission",
            request_id: "turn-split:permission-allowed",
            decision: "allow-once",
          },
          turnId: "turn-split",
          timestamp: "2026-07-07T12:00:02Z",
          state: "done",
        },
        text(
          "after-permission",
          "Session continued after approval.",
          "turn-split",
          "2026-07-07T12:00:03Z"
        ),
      ],
      { foldSettledTurns: true }
    );

    expect(rows.map(row => row.kind)).toEqual(["text", "data", "text"]);
    expect(rows.filter(row => row.kind === "text").map(row => row.part.text)).toEqual([
      "Streaming response started.",
      "Session continued after approval.",
    ]);
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
    expect(rows.map(row => row.kind)).toEqual(["live-tool", "text"]);
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
    expect(foldRow.cause).toBe("stopped");
    expect(foldRow.open).toBe(true);
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
    expect(foldRow.cause).toBe("stopped");
    expect(foldRow.open).toBe(true);
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
    expect(foldRow.cause).toBe("settled");
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
});

// UT-091: a settled turn folds behind its duration and its work counts; the
// final answer stays outside the fold.
it("Should fold a settled turn behind duration and counts with the answer outside", () => {
  const rows = deriveSessionRows(
    [
      tool(1, {
        toolName: "Bash",
        args: { command: "go test" },
        turnId: "turn-c",
        timestamp: "2026-07-07T12:00:00Z",
      }),
      tool(2, {
        toolName: "Bash",
        args: { command: "go vet" },
        turnId: "turn-c",
        timestamp: "2026-07-07T12:01:00Z",
      }),
      tool(3, {
        toolName: "Read",
        args: { file_path: "/src/a.ts" },
        turnId: "turn-c",
        timestamp: "2026-07-07T12:03:00Z",
      }),
      text("answer", "Done — 14 tests updated.", "turn-c", "2026-07-07T12:04:12Z"),
    ],
    { foldSettledTurns: true }
  );
  expect(rows.map(row => row.kind)).toEqual(["turn-fold", "text"]);
  const foldRow = rows[0];
  if (foldRow?.kind !== "turn-fold") throw new Error("expected fold row");
  expect(foldRow.label).toBe("Worked for 4m 12s · Ran 2 commands, read 1 file");
  expect(foldRow.counts).toBe("Ran 2 commands, read 1 file");
  expect(foldRow.cause).toBe("settled");
  expect(foldRow.open).toBe(false);
});

// UT-093: a zero-tool turn has no fold row — message and answer only.
it("Should derive no fold row for a zero-tool turn", () => {
  const rows = deriveSessionRows(
    [
      text(
        "only-answer",
        "Three of the manager tests are flaky.",
        "turn-z",
        "2026-07-07T12:00:00Z"
      ),
    ],
    { foldSettledTurns: true, activeTurnId: undefined }
  );
  expect(rows.map(row => row.kind)).toEqual(["text"]);
});

// UT-094: a turn a fallback steer interrupted folds like a settled one but
// names its true cause; a turn that failed stays open under a danger label.
it("Should fold a superseded turn with its true cause and keep a failed turn open", () => {
  const superseded = deriveSessionRows(
    [
      tool(1, {
        toolName: "Bash",
        args: { command: "go test" },
        turnId: "turn-s",
        timestamp: "2026-07-07T12:00:00Z",
      }),
      tool(3, {
        toolName: "Bash",
        args: { command: "go build" },
        turnId: "turn-s",
        timestamp: "2026-07-07T12:00:10Z",
      }),
      tool(2, {
        toolName: "Bash",
        args: { command: "go vet" },
        turnId: "turn-s",
        timestamp: "2026-07-07T12:00:20Z",
        status: "running",
        result: undefined,
      }),
      {
        kind: "reasoning",
        id: "reason-s",
        text: "Halfway",
        turnId: "turn-s",
        timestamp: "2026-07-07T12:00:48Z",
        state: "done",
      },
    ],
    { foldSettledTurns: true, supersededTurnIds: new Set(["turn-s"]) }
  );
  expect(superseded.map(row => row.kind)).toEqual(["turn-fold"]);
  const fold = superseded[0];
  if (fold?.kind !== "turn-fold") throw new Error("expected fold row");
  expect(fold.cause).toBe("steer_fallback");
  expect(fold.open).toBe(false);
  expect(fold.label).toBe("Interrupted after 48s · replaced by your steer · Ran 2 commands");
  // The call still running at the interrupt reads stopped, not running.
  const work = fold.rows.find(
    row => row.kind === "work" && row.entries.some(entry => entry.id === "tool-2")
  );
  if (work?.kind !== "work") throw new Error("expected the stopped call inside the fold");
  expect(work.entries.find(entry => entry.id === "tool-2")?.status).toBe("interrupted");

  const failed = deriveSessionRows(
    [
      tool(1, { turnId: "turn-f", timestamp: "2026-07-07T12:00:00Z" }),
      tool(2, { turnId: "turn-f", timestamp: "2026-07-07T12:02:03Z" }),
    ],
    { foldSettledTurns: true, failedTurnIds: new Set(["turn-f"]) }
  );
  const failedFold = failed[0];
  if (failedFold?.kind !== "turn-fold") throw new Error("expected fold row");
  expect(failedFold).toMatchObject({ cause: "failed", open: true, label: "Failed after 2m 3s" });
});

// UT-090: decision asks and errors are rich rows — they split tool groups and
// never disappear into a settled turn's fold.
it("Should keep rich rows out of tool groups and outside the fold", () => {
  const rows = deriveSessionRows(
    [
      tool(1, { turnId: "turn-r", timestamp: "2026-07-07T12:00:00Z" }),
      tool(2, { turnId: "turn-r", timestamp: "2026-07-07T12:00:01Z" }),
      {
        kind: "data",
        id: "error-r",
        name: "data-compozy-event",
        data: { type: "error", error: "provider exploded" },
        turnId: "turn-r",
        timestamp: "2026-07-07T12:00:02Z",
      },
      tool(3, { turnId: "turn-r", timestamp: "2026-07-07T12:00:03Z" }),
      tool(4, { turnId: "turn-r", timestamp: "2026-07-07T12:00:04Z" }),
      text("answer-r", "Recovered.", "turn-r", "2026-07-07T12:00:05Z"),
    ],
    { foldSettledTurns: true }
  );
  // The error row splits the run into two groups and stays visible beside the fold.
  expect(rows.map(row => row.kind)).toEqual(["turn-fold", "data", "text"]);
  const fold = rows[0];
  if (fold?.kind !== "turn-fold") throw new Error("expected fold row");
  expect(fold.rows.map(row => row.kind)).toEqual(["work", "work"]);
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
});

describe("marker clustering", () => {
  function markerEvent(
    id: string,
    kind: string,
    timestamp = "2026-07-07T12:00:00Z",
    turnId = "turn-1"
  ) {
    return {
      kind: "data",
      id,
      name: "data-compozy-event",
      // The daemon's wire shape: a `transcript_marker.created` event carrying the marker.
      data: {
        type: "transcript_marker.created",
        marker: { kind, occurred_at: timestamp, summary: kind },
      },
      turnId,
      timestamp,
      state: "done",
    } satisfies SessionTimelinePart;
  }

  it("Should merge consecutive same-kind markers into one row carrying the count", () => {
    const rows = deriveSessionRows([
      markerEvent("m1", "provider-retry"),
      markerEvent("m2", "provider-retry", "2026-07-07T12:00:01Z"),
      markerEvent("m3", "provider-retry", "2026-07-07T12:00:02Z"),
    ]);

    expect(rows).toHaveLength(1);
    const row = rows[0];
    if (row?.kind !== "data") throw new Error("expected data row");
    expect(row.count).toBe(3);
    expect(row.parts).toHaveLength(3);
    // The row anchors to the first event: stable id + render anchor part.
    expect(row.id).toBe("data:m1");
    expect(row.part.id).toBe("m1");
  });

  it("Should split marker clusters when the kind changes", () => {
    const rows = deriveSessionRows([
      markerEvent("m1", "provider-retry"),
      markerEvent("m2", "compaction"),
    ]);

    expect(rows).toHaveLength(2);
    expect(rows.map(row => (row.kind === "data" ? row.count : row.kind))).toEqual([1, 1]);
  });

  it("Should split same-kind marker clusters when the turn changes", () => {
    const rows = deriveSessionRows([
      markerEvent("m1", "provider-retry", "2026-07-07T12:00:00Z", "turn-1"),
      markerEvent("m2", "provider-retry", "2026-07-07T12:00:01Z", "turn-2"),
    ]);

    expect(rows).toHaveLength(2);
    expect(rows.map(row => (row.kind === "data" ? row.parts.map(part => part.id) : []))).toEqual([
      ["m1"],
      ["m2"],
    ]);
  });

  it("Should never cluster permission or clarification parts", () => {
    const permission = (id: string): SessionTimelinePart => ({
      kind: "data",
      id,
      name: "data-compozy-permission",
      data: { type: "permission", request_id: id, decision: "allow-once" },
      turnId: "turn-1",
      timestamp: "2026-07-07T12:00:00Z",
      state: "done",
    });
    const clarify = (id: string): SessionTimelinePart => ({
      kind: "data",
      id,
      name: "data-compozy-event",
      data: { type: "clarify", status: "pending", request: { request_id: id, question: "?" } },
      turnId: "turn-1",
      timestamp: "2026-07-07T12:00:00Z",
      state: "done",
    });

    const rows = deriveSessionRows([
      permission("p1"),
      permission("p2"),
      clarify("c1"),
      clarify("c2"),
    ]);

    expect(rows).toHaveLength(4);
    for (const row of rows) {
      if (row.kind !== "data") throw new Error("expected data rows");
      expect(row.count).toBe(1);
    }
  });
});
