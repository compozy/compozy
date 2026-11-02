import { fn } from "storybook/test";

import type { CheckInsEditorModel } from "../hooks/use-check-ins-editor";
import type { EditorDocument } from "../hooks/use-editor-document";
import type { MemoryEditorModel } from "../hooks/use-memory-editor";
import {
  blankCheckIns,
  composeCheckInsFile,
  type CheckInSettings,
} from "../lib/agent-check-in-form";
import {
  composeMemoryFile,
  memoryFullness,
  memoryPercent,
  parseMemoryFile,
  type MemoryFile,
} from "../lib/agent-memory-notes";
import { composeSoulFile, parseSoulFile, type SoulForm } from "../lib/agent-soul-form";
import type { AgentMemoryBudget } from "../types";
import { agentInstanceFixture } from "./instance-fixtures";

/** A still document for the pure editor views: what a loaded file looks like, with no I/O. */
export function editorDocumentFixture<TForm>(
  form: TForm,
  source: string,
  overrides: Partial<EditorDocument<TForm>> = {}
): EditorDocument<TForm> {
  return {
    baseline: { digest: "sha256:fixture", source, form, block: null },
    form,
    raw: null,
    conflict: null,
    saving: false,
    problem: undefined,
    source,
    block: null,
    changes: 0,
    setForm: fn(),
    setRaw: fn(),
    discard: fn(),
    save: fn(async () => undefined),
    chooseTheirs: fn(),
    ...overrides,
  };
}

const SOUL_FILE = `---
version: "1"
tone:
  - "direct"
constraints:
  - "Merge anything on its own"
  - "Close an issue without asking"
---
Keep this project's issues moving.

Every morning, look at what's open and work out what's stuck. Name who can unblock it, and suggest the next step.

- Hand building work to **Code Engineer**
- Hand reviews to **Reviewer**
- Stay quiet when nothing changed
`;

export function soulDocumentFixture(overrides: Partial<EditorDocument<SoulForm>> = {}) {
  const { form } = parseSoulFile(SOUL_FILE);
  return editorDocumentFixture(form, composeSoulFile(form), overrides);
}

export function blankSoulDocumentFixture() {
  const { form } = parseSoulFile("");
  const blank: SoulForm = { ...form, tone: "direct" };
  return editorDocumentFixture(blank, "", {
    baseline: { digest: "", source: "", form: blank, block: null, missing: true },
  });
}

const CHECK_IN_SETTINGS: CheckInSettings = {
  ...blankCheckIns({ defaultStop: 2, timezone: "UTC" }),
  hours: [9, 18],
  lunch: true,
  looksAt: ["task", "session_health", "sessions_idle"],
};

export function checkInsModelFixture(
  overrides: Partial<CheckInsEditorModel> = {}
): CheckInsEditorModel {
  const instance = agentInstanceFixture();
  return {
    document: editorDocumentFixture(CHECK_IN_SETTINGS, composeCheckInsFile(CHECK_IN_SETTINGS)),
    settings: CHECK_IN_SETTINGS,
    floor: 1,
    limit: false,
    setLimit: fn(),
    readOnly: false,
    nowHour: 14.2,
    instance,
    places: [instance, agentInstanceFixture({ id: "ainst-advos", scope: "advos" })],
    setPlace: fn(),
    standing: "running",
    nowhere: false,
    never: false,
    on: true,
    paused: false,
    pauseCopy: undefined,
    resumeBlocked: false,
    failedInARow: 0,
    slowedBy: null,
    next: { day: "Today", time: "14:30", distance: "in 18 min" },
    activity: {
      past: [
        { at: "2026-11-01T09:00:00Z", cell: "quiet" },
        { at: "2026-11-01T09:30:00Z", cell: "told" },
        { at: "2026-11-01T14:00:00Z", cell: "quiet" },
      ],
      recent: [
        {
          key: "a",
          time: "14:00",
          title: "All quiet",
          description: "Nothing new, so it didn't bother you.",
        },
        {
          key: "b",
          time: "09:30",
          title: "Told you something",
          description: "Token migration is stuck.",
          told: true,
        },
      ],
    },
    tryCheck: { state: { phase: "idle" }, tryIt: fn(), runForReal: fn(), reset: fn() },
    turnOn: fn(),
    resume: fn(),
    ...overrides,
  };
}

export function blankCheckInsModelFixture() {
  const instance = agentInstanceFixture({ checks_enabled: false, last_wake_at: "" });
  return checkInsModelFixture({
    instance,
    places: [instance],
    standing: "off",
    never: true,
    on: false,
  });
}

const MEMORY_FILE = `- 2026-11-02 · reloaddev · Preview deploys need \`make preview\`. The normal deploy skips the staging data. · until 2026-12-01
- 2026-10-31 · compozy · Give each issue its own worker. If something urgent comes up, steer the worker that's already on it.
Keep status updates short. Two lines is usually enough.
- 2026-10-28 · advos · Hearing dates live in the calendar, not in tasks.
Talk to me in Brazilian Portuguese. Code and docs stay in English.
`;

function memoryModel(content: string): MemoryEditorModel {
  const form: MemoryFile = parseMemoryFile(content);
  const budget: AgentMemoryBudget = {
    lines: form.notes.length,
    max_lines: 200,
    bytes: new TextEncoder().encode(content).length,
    max_bytes: 24_576,
  };
  const learned = form.notes.filter(note => note.learnedIn).length;
  const percent = memoryPercent(budget);
  const fullness = memoryFullness(percent, learned);
  return {
    document: editorDocumentFixture(form, composeMemoryFile(form)),
    budget,
    today: "2026-11-01",
    notes: form.notes,
    saved: form.notes,
    adding: false,
    seed: "",
    search: "",
    filter: "all",
    percent,
    fullness,
    full: fullness === "full",
    learned,
    shown: [...form.notes],
    revisions:
      form.notes.length > 0
        ? [
            { id: "rev-2", name: "Version 2" },
            { id: "rev-1", name: "Version 1" },
          ]
        : [],
    restoring: false,
    setSearch: fn(),
    setFilter: fn(),
    startAdding: fn(),
    cancelAdding: fn(),
    add: fn(),
    update: fn(),
    remove: fn(),
    restore: fn(),
  };
}

export const memoryModelFixture = () => memoryModel(MEMORY_FILE);
export const blankMemoryModelFixture = () => memoryModel("");
