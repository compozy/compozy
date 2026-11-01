// Invariant: each editor's form reads and writes its file without losing anything, and a file the
// form can't hold is refused rather than rewritten.
// Owner: the pure file libs; canonical suite for UT-076 and the parsing half of UT-078.
import { describe, expect, it } from "vitest";

import { checkInActivity, checkInReason } from "../agent-check-in-activity";
import {
  LOOK_AT_IDS,
  blankCheckIns,
  composeCheckInsFile,
  floorStop,
  parseCheckInsFile,
  previewNextCheckIn,
  scheduleSummary,
  type CheckInSettings,
} from "../agent-check-in-form";
import {
  fileBlockCopy,
  fileProblem,
  parseFrontmatter,
  splitFrontmatter,
} from "../agent-frontmatter";
import {
  composeMemoryFile,
  formatMemoryDate,
  memoryFullness,
  memoryNoteExpired,
  memoryPercent,
  parseMemoryFile,
} from "../agent-memory-notes";
import { BLANK_SOUL, composeSoulFile, parseSoulFile } from "../agent-soul-form";

const DEFAULTS = { defaultStop: 2, timezone: "America/Sao_Paulo" };

const CHECK_INS_FILE = `---
version: 1
enabled: true
summary: "Keeps issues moving"
preferences:
  min_interval: 1h
  active_hours:
    - timezone: America/Sao_Paulo
      start: "09:00"
      end: "19:00"
  quiet_windows:
    - timezone: America/Sao_Paulo
      start: "12:00"
      end: "13:00"
context:
  include: [task, session_health, monitors_fired]
---
Flag blocked tasks and PRs waiting on me.
`;

function settingsOf(source: string): CheckInSettings {
  const parsed = parseCheckInsFile(source, DEFAULTS);
  expect(parsed.block).toBeNull();
  return parsed.settings;
}

describe("HEARTBEAT.md form", () => {
  it("Should read the file's keys into the form", () => {
    expect(settingsOf(CHECK_INS_FILE)).toMatchObject({
      stop: 3,
      hours: [9, 19],
      lunch: true,
      looksAt: ["task", "session_health", "monitors_fired"],
      also: "Flag blocked tasks and PRs waiting on me.",
      timezone: "America/Sao_Paulo",
      kept: { version: 1, enabled: true, summary: "Keeps issues moving" },
    });
  });

  it("Should round-trip every form field through the file", () => {
    const settings = settingsOf(CHECK_INS_FILE);
    const edited: CheckInSettings = {
      ...settings,
      stop: 1,
      hours: [8, 24],
      lunch: false,
      looksAt: ["automations_failed"],
      also: "Watch the **nightly** build.",
    };
    const reread = settingsOf(composeCheckInsFile(edited));
    expect({ ...reread, kept: null }).toEqual({
      ...edited,
      kept: null,
    });
    expect(reread.kept.summary).toBe("Keeps issues moving");
  });

  it("Should keep the file's own interval spelling until the slider moves", () => {
    const composed = composeCheckInsFile(settingsOf(CHECK_INS_FILE));
    expect(composed).toContain('min_interval: "1h"');
    expect(composeCheckInsFile({ ...settingsOf(CHECK_INS_FILE), stop: 4 })).toContain(
      'min_interval: "120m"'
    );
  });

  it("Should not invent keys a file left out", () => {
    const source = "---\nversion: 1\nenabled: true\n---\nJust watch things.\n";
    const composed = composeCheckInsFile(settingsOf(source));
    expect(composed).toBe(source);
  });

  it.each([
    ["two active windows", `---\npreferences:\n  active_hours:\n    - {timezone: UTC}\n---\n`],
    [
      "an overnight window",
      `---\npreferences:\n  active_hours:\n    - timezone: UTC\n      start: "22:00"\n      end: "06:00"\n---\n`,
    ],
    ["an interval between stops", `---\npreferences:\n  min_interval: 45m\n---\n`],
    [
      "a custom quiet window",
      `---\npreferences:\n  quiet_windows:\n    - timezone: UTC\n      start: "15:00"\n      end: "16:00"\n---\n`,
    ],
    ["an unknown context source", `---\ncontext:\n  include: [weather]\n---\n`],
    ["a comment", `---\n# keep this\nenabled: true\n---\n`],
  ])("Should refuse %s", (_name, source) => {
    expect(parseCheckInsFile(source, DEFAULTS).block).toBe("settings");
  });

  it("Should show every source and all day when the file says nothing about them", () => {
    expect(settingsOf("---\nversion: 1\nenabled: true\n---\n")).toMatchObject({
      hours: [0, 24],
      lunch: false,
      looksAt: LOOK_AT_IDS,
    });
  });

  it("Should describe no file the way the daemon runs it", () => {
    const form = blankCheckIns({ defaultStop: 2, timezone: "UTC" });
    expect(form).toMatchObject({ hours: [0, 24], lunch: false, looksAt: LOOK_AT_IDS });
    expect(scheduleSummary(form, true)).toBe("Every 30 min, all day");
  });

  it("Should keep every source off through a save instead of falling back to all five", () => {
    const form = settingsOf(CHECK_INS_FILE);
    const body = composeCheckInsFile({ ...form, looksAt: [] });
    expect(body).toContain("include: []");
    expect(settingsOf(body).looksAt).toEqual([]);
  });

  it("Should refuse guidance the Markdown field would flatten", () => {
    expect(parseCheckInsFile("---\nversion: 1\n---\n# Heading\n", DEFAULTS).block).toBe("heading");
  });
});

describe("the floor and the next check-in", () => {
  it("Should stop at the configured floor, or the first stop without one", () => {
    expect(floorStop("15m")).toBe(1);
    expect(floorStop("20m")).toBe(2);
    expect(floorStop(undefined)).toBe(0);
  });

  it("Should preview from the real clock and skip lunch", () => {
    const settings = settingsOf(CHECK_INS_FILE);
    expect(previewNextCheckIn(settings, 11.5)).toEqual({
      day: "Today",
      time: "13:00",
      distance: "in 1 hr 30 min",
    });
    expect(previewNextCheckIn(settings, 18.5)).toEqual({ day: "Tomorrow", time: "09:00" });
  });
});

describe("SOUL.md form", () => {
  const file = `---
version: "1"
role: "Issue triager"
tone:
  - "friendly"
principles:
  - "Stay quiet"
constraints:
  - "Merge anything on its own"
tags:
  - "ops"
---
Keep this project's issues moving.

- Hand reviews to **Reviewer**
`;

  it("Should keep keys the page doesn't edit through a save", () => {
    const { form, block } = parseSoulFile(file);
    expect(block).toBeNull();
    expect(form).toMatchObject({ tone: "friendly", never: ["Merge anything on its own"] });
    expect(composeSoulFile(form)).toBe(file);
  });

  it("Should map an unlisted tone to Your own and back", () => {
    const { form } = parseSoulFile(`---\ntone: "Calm and dry"\n---\nBody\n`);
    expect(form).toMatchObject({ tone: "custom", customTone: "Calm and dry" });
    expect(composeSoulFile(form)).toContain('- "Calm and dry"');
  });

  it("Should leave tone out of a file that never set one", () => {
    const { form } = parseSoulFile("Only a body\n");
    expect(form.tone).toBeNull();
    expect(composeSoulFile(form)).not.toContain("tone");
  });

  it("Should give a new file a version and the page's default tone", () => {
    expect(composeSoulFile({ ...BLANK_SOUL, body: "Do it" })).toBe(
      '---\nversion: "1"\ntone:\n  - "direct"\n---\nDo it\n'
    );
  });

  it.each([
    ["table", "---\nversion: 1\n---\n| Step | Who |\n| --- | --- |\n| Tag | Ana |\n"],
    ["heading", "# Soul\n\nDefine persona.\n"],
    ["quote", "> Be kind\n"],
    ["code", "```\nmake\n```\n"],
    ["settings", "---\nfavorite_color: blue\n---\nBody\n"],
    ["settings", "---\ntone:\n  - direct\n  - friendly\n---\nBody\n"],
    ["mistake", "---\ntone: direct\n"],
  ])("Should open a file with %s in Advanced only", (block, source) => {
    expect(parseSoulFile(source).block).toBe(block);
  });

  it("Should say why, in plain words", () => {
    expect(fileBlockCopy("table")).toBe(
      "This file has a table the editor can't show, so it opens here. Nothing gets lost."
    );
  });

  it("Should turn a server diagnostic into page copy", () => {
    expect(
      fileProblem([
        { message: "ignored", severity: "warning" },
        { message: "SOUL.md frontmatter is not valid YAML", line: 2, severity: "error" },
      ])
    ).toBe("Line 2: This file is not valid YAML");
  });
});

describe("frontmatter subset", () => {
  it("Should read nested maps, lists of maps and flow lists", () => {
    const parts = splitFrontmatter(CHECK_INS_FILE);
    expect(parseFrontmatter(parts?.metadata ?? "")).toMatchObject({
      preferences: {
        min_interval: "1h",
        active_hours: [{ timezone: "America/Sao_Paulo", start: "09:00", end: "19:00" }],
      },
      context: { include: ["task", "session_health", "monitors_fired"] },
    });
  });

  it("Should read a file without frontmatter as all body", () => {
    expect(splitFrontmatter("Hello")).toEqual({ metadata: null, body: "Hello" });
  });

  it.each(["key: |\n  block", "key: &anchor value", "a: 1\na: 2", "\tkey: value"])(
    "Should give up on %j",
    metadata => {
      expect(parseFrontmatter(metadata)).toBeNull();
    }
  );
});

describe("MEMORY.md notes", () => {
  const file = `- 2026-10-31 · compozy · Dispatch one executor per issue.
- 2026-11-02 · reloaddev · Preview deploys · need \`make preview\`. · until 2026-12-01
Prefer short status updates.

- 2026-11-01 · you · Ana is out. · until 2026-11-10
`;

  it("Should tell learned lines from the person's own", () => {
    const { notes } = parseMemoryFile(file);
    expect(notes).toHaveLength(4);
    expect(notes[0]).toMatchObject({ learnedIn: "compozy", date: "2026-10-31" });
    expect(notes[1]).toMatchObject({
      learnedIn: "reloaddev",
      text: "Preview deploys · need `make preview`.",
      until: "2026-12-01",
    });
    expect(notes[2]).toMatchObject({ text: "Prefer short status updates." });
    expect(notes[2]?.learnedIn).toBeUndefined();
    expect(notes[3]).toMatchObject({ until: "2026-11-10" });
    expect(notes[3]?.learnedIn).toBeUndefined();
  });

  it("Should write an untouched file back byte for byte", () => {
    expect(composeMemoryFile(parseMemoryFile(file))).toBe(file);
  });

  it("Should change only the edited line", () => {
    const parsed = parseMemoryFile(file);
    const notes = parsed.notes.map((note, index) =>
      index === 0 ? { ...note, text: "One worker per issue.", line: undefined } : note
    );
    expect(composeMemoryFile({ ...parsed, notes })).toBe(
      file.replace("Dispatch one executor per issue.", "One worker per issue.")
    );
  });

  it("Should treat a bad date as the person's own line", () => {
    const { notes } = parseMemoryFile("- 2026-13-45 · compozy · Not a date\n");
    expect(notes[0]?.learnedIn).toBeUndefined();
  });

  it("Should expire a note only after its until day", () => {
    expect(memoryNoteExpired({ until: "2026-11-01" }, "2026-11-01")).toBe(false);
    expect(memoryNoteExpired({ until: "2026-10-31" }, "2026-11-01")).toBe(true);
  });

  it("Should show dates plainly", () => {
    expect(formatMemoryDate("2026-11-02", "2026-11-01")).toBe("Nov 2");
    expect(formatMemoryDate("2025-11-02", "2026-11-01")).toBe("Nov 2, 2025");
  });

  it("Should read fullness from the larger of lines and bytes", () => {
    const budget = { lines: 20, max_lines: 200, bytes: 23_000, max_bytes: 24_576 };
    expect(memoryPercent(budget)).toBe(94);
    expect(memoryFullness(94, 3)).toBe("almost");
    expect(memoryFullness(89, 3)).toBe("fine");
    expect(memoryFullness(100, 3)).toBe("almost");
    expect(memoryFullness(100, 0)).toBe("full");
  });
});

describe("check-in activity", () => {
  const now = new Date(2026, 10, 1, 14, 12);
  const at = (hour: number, minute = 0) => new Date(2026, 10, 1, hour, minute).toISOString();

  it("Should fold quiet runs and keep what mattered", () => {
    const { recent, past } = checkInActivity(
      [
        { result: "quiet", reason: "digest_empty", at: at(9) },
        { result: "quiet", reason: "digest_unchanged", at: at(9, 30) },
        { result: "reported", reason: "reported", detail: "Tokens are stuck.", at: at(10) },
        { result: "failed", reason: "triage_timeout", at: at(10, 30) },
        { result: "quiet", reason: "digest_empty", at: new Date(2026, 9, 31, 9).toISOString() },
      ],
      now
    );
    expect(past.map(item => item.cell)).toEqual(["quiet", "quiet", "told", "failed"]);
    expect(recent.map(item => item.title)).toEqual([
      "Couldn't finish",
      "Told you something",
      "2 quiet check-ins",
    ]);
    expect(recent[0]?.description).toBe("It ran out of time.");
    expect(recent[1]?.description).toBe("Tokens are stuck.");
  });

  it("Should turn every reason into a sentence, and an unknown one into a safe default", () => {
    expect(checkInReason("session_prompt_active")).toBe(
      "It was busy in its main chat, so it skipped this one."
    );
    expect(checkInReason("brand_new_code")).toBe("It skipped this one.");
  });
});
