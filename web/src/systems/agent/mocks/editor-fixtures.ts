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
import { pendingScriptFixture } from "./script-approval-fixtures";

/** A still document for the pure editor views: what a loaded file looks like, with no I/O. */
export function editorDocumentFixture<TForm>(
  form: TForm,
  source: string,
  overrides: Partial<EditorDocument<TForm>> = {}
): EditorDocument<TForm> {
  return {
    resourceKey: "fixture",
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
    catalog: null,
    sources: {
      catalogState: "loading",
      retryCatalog: fn(),
      save: fn(),
      remove: fn(),
      approve: fn(),
      approving: null,
      openExtensions: fn(),
      openScript: fn(),
    },
    authority: {
      current: { digest: "", status: "none", check_ins: {}, actions: [] },
      writeTools: [
        {
          tool_id: "compozy__notify",
          title: "Notify",
          access: "write",
          check_in_writable: true,
          backend: "native_go",
        },
        {
          tool_id: "mcp__github__create_issue_comment",
          title: "Comment on an issue",
          access: "write",
          check_in_writable: true,
          backend: "mcp",
          server: "github",
        },
      ],
      toolsState: "ready",
      approve: fn(),
      approving: false,
      confirmSave: async () => false,
      pending: null,
      writeWarnings: [],
    },
    now: new Date("2026-11-01T14:12:00Z"),
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

const MEMORY_FILE = `- [rule] File issues through report-writer.
- 2026-11-02 · reloaddev · Preview deploys need \`make preview\`. The normal deploy skips the staging data. · until 2026-12-01
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
    setKind: fn(),
    remove: fn(),
    restore: fn(),
  };
}

export const memoryModelFixture = () => memoryModel(MEMORY_FILE);
export const blankMemoryModelFixture = () => memoryModel("");

/**
 * Heartbeat with a script source the operator hasn't approved and no check has read (#1258):
 * the instance still reads `pending_first_poll`, the catalog says the bytes wait.
 */
export function awaitingApprovalCheckInsModelFixture({ paused = false } = {}) {
  const settings: CheckInSettings = {
    ...CHECK_IN_SETTINGS,
    sources: [
      { id: "issues", kind: "forge-github/issues", with: { repo: "compozy/compozy" } },
      { id: "merges", script: pendingScriptFixture.name },
    ],
  };
  const pending = { status: "pending_first_poll", failure_streak: 0, last_item_count: 0 };
  const instance = agentInstanceFixture({
    ...(paused ? { state: "paused", pause_reason: "manual" } : {}),
    sources: [
      { ...pending, id: "issues", kind: "forge-github/issues" },
      {
        ...pending,
        id: "merges",
        script: pendingScriptFixture.name,
        awaiting_approval: true,
        script_sha256: pendingScriptFixture.sha256,
      },
    ],
  });
  const base = checkInsModelFixture();
  return checkInsModelFixture({
    document: editorDocumentFixture(settings, composeCheckInsFile(settings)),
    settings,
    instance,
    places: [instance],
    ...(paused ? { standing: "paused", paused: true, on: false } : {}),
    catalog: {
      agent: "merge-train",
      scripts_enabled: true,
      limits: { max_per_agent: 8, max_items: 20 },
      kinds: [],
      scripts: [
        {
          ...pendingScriptFixture,
          path: "/Users/you/.compozy/agents/merge-train/sources/merge-candidates",
        },
      ],
    },
    sources: { ...base.sources, catalogState: "ready" },
  });
}

const AUTHORITY_SETTINGS: CheckInSettings = {
  ...CHECK_IN_SETTINGS,
  authority: {
    writes: ["mcp__github__add_label"],
    caps: {},
    actions: [
      {
        id: "triage",
        title: "Triage new issues",
        instructions: "Label each new issue and say which ones need me.",
        autoAccept: true,
      },
      {
        id: "close-stale",
        title: "Close a stale issue",
        instructions: "Close it with a short note that links the fix.",
        autoAccept: false,
      },
    ],
  },
};

const AUTHORITY_BLOCK = {
  digest: "sha256:3f9c",
  status: "unapproved",
  check_ins: { allow_writes: ["mcp__github__add_label"] },
  actions: [
    {
      id: "triage",
      title: "Triage new issues",
      instructions: "Label each new issue and say which ones need me.",
      auto_accept: true,
    },
  ],
};

/**
 * U6/U7: a Heartbeat page with check-in writes and actions. `waiting` shows the approval notice;
 * `confirming` holds a save on the confirmation that lists the block.
 */
export function authorityCheckInsModelFixture({
  waiting = false,
  confirming = false,
}: { waiting?: boolean; confirming?: boolean } = {}) {
  const base = checkInsModelFixture();
  return checkInsModelFixture({
    document: editorDocumentFixture(AUTHORITY_SETTINGS, composeCheckInsFile(AUTHORITY_SETTINGS)),
    settings: AUTHORITY_SETTINGS,
    authority: {
      ...base.authority,
      current: waiting ? AUTHORITY_BLOCK : { ...AUTHORITY_BLOCK, status: "approved" },
      pending: confirming ? { authority: AUTHORITY_BLOCK, settle: fn() } : null,
    },
  });
}
