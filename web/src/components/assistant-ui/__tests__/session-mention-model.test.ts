import { describe, expect, it } from "vitest";

import {
  readSessionMentionRecents,
  sessionMentionRecentsKey,
  withSessionMentionRecent,
  writeSessionMentionRecents,
} from "@/systems/session";

import { createSessionCommandFormatter } from "../session-command-formatter";
import { agentMentionDirectives, sessionMentionDirectives } from "../session-directive-registry";
import {
  buildMentionMenuModel,
  detectMentionTrigger,
  emphasizeMatch,
  fileMentionItem,
  mentionDirectiveFormatter,
  parseMentionDirectiveId,
  pathMentionToken,
  sessionMentionItem,
  type MentionMenuSources,
} from "../session-mention-model";
import {
  activeMentionTrigger,
  remapDismissedOffsets,
  sessionMentionTriggerLogic,
} from "../session-mention-trigger-store";

const EMPTY_CATALOG = { standaloneSections: [], inlineSkills: [] };

function sources(overrides: Partial<MentionMenuSources> = {}): MentionMenuSources {
  return {
    query: "",
    files: { entries: [], status: "ready", truncated: false },
    sessions: { sessions: [], status: "ready" },
    recents: [],
    ...overrides,
  };
}

describe("detectMentionTrigger", () => {
  it.each([
    ["@", 1, { offset: 0, query: "" }],
    ["Review @clie", 12, { offset: 7, query: "clie" }],
    ["line\n@web/", 10, { offset: 5, query: "web/" }],
    ["Review @clie tail", 12, { offset: 7, query: "clie" }],
  ])("Should open on %j at the caret", (text, cursor, expected) => {
    expect(detectMentionTrigger(text, cursor)).toEqual(expected);
  });

  it.each([
    ["me@example.com", 14],
    ["@clie done", 10],
    ["plain text", 10],
  ])("Should stay closed for %j", (text, cursor) => {
    expect(detectMentionTrigger(text, cursor)).toBeNull();
  });
});

describe("mention tokens", () => {
  it("Should serialize file, folder, quoted and session picks to the daemon token", () => {
    expect(
      mentionDirectiveFormatter.serialize(
        fileMentionItem({ path: "internal/acp/client_prompt.go", kind: "file" })
      )
    ).toBe("@internal/acp/client_prompt.go");
    expect(
      mentionDirectiveFormatter.serialize(fileMentionItem({ path: "web/src/", kind: "folder" }))
    ).toBe("@web/src/");
    expect(
      mentionDirectiveFormatter.serialize(fileMentionItem({ path: "web/src", kind: "folder" }))
    ).toBe("@web/src/");
    expect(
      mentionDirectiveFormatter.serialize(
        fileMentionItem({ path: "docs/My Notes.md", kind: "file" })
      )
    ).toBe('@"docs/My Notes.md"');
    expect(
      mentionDirectiveFormatter.serialize(
        sessionMentionItem({
          id: "sess-a",
          name: "Fix login bug",
          agent_name: "codex",
          updated_at: "",
        })
      )
    ).toBe("@session:sess-a");
  });

  it("Should quote a path whose end the daemon would trim as punctuation", () => {
    expect(pathMentionToken("notes/v1.")).toBe('@"notes/v1."');
    expect(pathMentionToken("src/a.ts")).toBe("@src/a.ts");
  });

  it("Should quote a path whose leading characters would change the token's meaning", () => {
    expect(pathMentionToken("@notes.md")).toBe('@"@notes.md"');
    expect(pathMentionToken("session:notes.md")).toBe('@"session:notes.md"');
    expect(pathMentionToken("docs/session:notes.md")).toBe("@docs/session:notes.md");
  });

  it("Should label chips by name, folder with a slash, untitled session by id", () => {
    expect(fileMentionItem({ path: "internal/acp/client_prompt.go", kind: "file" }).label).toBe(
      "client_prompt.go"
    );
    expect(fileMentionItem({ path: "web/src/", kind: "folder" }).label).toBe("src/");
    expect(
      sessionMentionItem({ id: "sess-9", name: " ", agent_name: "codex", updated_at: "" }).label
    ).toBe("sess-9");
  });

  it("Should round-trip the chip kind and target through the directive id", () => {
    const item = fileMentionItem({ path: "a b/c.md", kind: "file" });
    expect(parseMentionDirectiveId(item.id)).toEqual({ kind: "file", target: "a b/c.md" });
    expect(parseMentionDirectiveId("session:sess-a")).toEqual({
      kind: "session",
      target: "sess-a",
    });
    expect(parseMentionDirectiveId("skill:/review")).toBeNull();
  });

  it("Should keep typed @ text plain while serializing picked mentions in the composer formatter", () => {
    const formatter = createSessionCommandFormatter(EMPTY_CATALOG);
    expect(formatter.parse("Check @README.md")).toEqual([
      { kind: "text", text: "Check @README.md" },
    ]);
    expect(mentionDirectiveFormatter.parse("Check @README.md")).toEqual([
      { kind: "text", text: "Check @README.md" },
    ]);
    expect(formatter.serialize(fileMentionItem({ path: "docs/My Notes.md", kind: "file" }))).toBe(
      '@"docs/My Notes.md"'
    );
  });
});

describe("buildMentionMenuModel", () => {
  it("Should lead a bare @ with recents, then the daemon's shallow paths, without duplicates", () => {
    const model = buildMentionMenuModel(
      sources({
        recents: [{ path: "internal/acp/client_prompt.go", kind: "file" }],
        files: {
          entries: [
            { path: "README.md", kind: "file" },
            { path: "internal/acp/client_prompt.go", kind: "file" },
          ],
          status: "ready",
          truncated: false,
        },
      })
    );
    expect(model.items.map(item => item.id)).toEqual([
      "file:internal/acp/client_prompt.go",
      "file:README.md",
    ]);
  });

  it("Should ignore recents once a query is typed and cap files at 50 rows", () => {
    const entries = Array.from({ length: 60 }, (_, index) => ({
      path: `f${index}.ts`,
      kind: "file" as const,
    }));
    const model = buildMentionMenuModel(
      sources({
        query: "f",
        recents: [{ path: "old.md", kind: "file" }],
        files: { entries, status: "ready", truncated: true },
      })
    );
    expect(model.items).toHaveLength(50);
    expect(model.items[0]!.id).toBe("file:f0.ts");
    expect(model.groups[0]!.truncated).toBe(true);
  });

  it("Should give each group its own empty, error and loading message", () => {
    const model = buildMentionMenuModel(
      sources({
        query: "zz",
        files: { entries: [{ path: "a.ts", kind: "file" }], status: "error", truncated: false },
        sessions: { sessions: [], status: "ready" },
      })
    );
    expect(model.groups.map(group => group.message)).toEqual([
      "Couldn't search files",
      "No matching sessions",
    ]);
    expect(model.items).toEqual([]);

    const failing = buildMentionMenuModel(sources({ sessions: { sessions: [], status: "error" } }));
    expect(failing.groups.map(group => group.message)).toEqual([
      "No matching files",
      "Couldn't load sessions",
    ]);

    const loading = buildMentionMenuModel(
      sources({ files: { entries: [], status: "loading", truncated: false } })
    );
    expect(loading.groups[0]!.message).toBe("Loading files…");
    expect(loading.loading).toBe(true);
  });

  it("Should number session rows after file rows", () => {
    const model = buildMentionMenuModel(
      sources({
        files: { entries: [{ path: "a.ts", kind: "file" }], status: "ready", truncated: false },
        sessions: {
          sessions: [{ id: "sess-a", name: "A", agent_name: "codex", updated_at: "" }],
          status: "ready",
        },
      })
    );
    expect(model.groups[1]!.rows.map(row => row.flatIndex)).toEqual([1]);
  });

  it("Should emphasize the first case-insensitive match", () => {
    expect(emphasizeMatch("client_prompt.go", "PROMPT")).toEqual([
      { text: "client_", match: false },
      { text: "prompt", match: true },
      { text: ".go", match: false },
    ]);
    expect(emphasizeMatch("main.go", "zz")).toEqual([{ text: "main.go", match: false }]);
  });
});

// Invariant: an agent mention lists with its label and reaches the prompt as slug plus name, in a
// form the daemon leaves as text. Owner: mention model; canonical suite: this file.
describe("agent mentions", () => {
  const agent = { name: "code_implementer", label: "Builder", resolved_appearance: null } as never;
  const empty = {
    query: "",
    files: { entries: [], status: "ready" as const, truncated: false },
    sessions: { sessions: [], status: "ready" as const },
    recents: [],
  };

  it("Should list agents after the other groups, numbered after their rows", () => {
    const model = buildMentionMenuModel({
      ...empty,
      files: { ...empty.files, entries: [{ path: "a.md", kind: "file" }] },
      agents: { agents: [agent], status: "ready" },
    });

    expect(model.groups.map(group => group.id)).toEqual(["files", "sessions", "agents"]);
    const row = model.groups[2]!.rows[0]!;
    expect(row.flatIndex).toBe(1);
    expect(row.item).toMatchObject({ id: "agent:code_implementer", label: "Builder" });
  });

  it("Should serialize slug and display name and read them back from sent text", () => {
    const item = buildMentionMenuModel({
      ...empty,
      agents: { agents: [agent], status: "ready" },
    }).items[0]!;
    const token = mentionDirectiveFormatter.serialize(item);

    expect(token).toBe("@agent:code_implementer (Builder)");
    expect(parseMentionDirectiveId(item.id)).toEqual({
      kind: "agent",
      target: "code_implementer",
    });
    expect(agentMentionDirectives(`Ask ${token} first`)).toEqual([
      {
        kind: "agent",
        token,
        target: "code_implementer",
        label: "Builder",
        start: 4,
        end: 4 + token.length,
      },
    ]);
  });

  it("Should leave an address that only looks like a mention as text", () => {
    expect(agentMentionDirectives("mail me@agent:x (Y)")).toEqual([]);
  });
});

describe("sessionMentionDirectives", () => {
  const mention = { kind: "file", token: "@a.ts", target: "a.ts", label: "a.ts", start: 0, end: 5 };

  it("Should read mentions at the top level and under custom", () => {
    expect(sessionMentionDirectives({ mentions: [mention] })).toEqual([mention]);
    expect(sessionMentionDirectives({ custom: { mentions: [mention] } })).toEqual([mention]);
  });

  it("Should ignore missing and malformed entries", () => {
    expect(sessionMentionDirectives({})).toEqual([]);
    expect(sessionMentionDirectives(null)).toEqual([]);
    expect(
      sessionMentionDirectives({
        mentions: [
          { ...mention, kind: "agent" },
          { ...mention, token: "a.ts" },
          { ...mention, start: 5, end: 5 },
          { ...mention, target: "" },
          "@a.ts",
        ],
      })
    ).toEqual([]);
  });

  it("Should fall back to the target when no label was recorded", () => {
    expect(sessionMentionDirectives({ mentions: [{ ...mention, label: "" }] })[0]!.label).toBe(
      "a.ts"
    );
  });
});

describe("mention trigger store", () => {
  // Invariant (US-004.AC-1/2, VC-10): a dismissal belongs to its `@` and lasts
  // as long as that `@` exists — typing after it, leaving and returning with
  // the caret, and edits elsewhere never reopen it; a new `@` opens.
  it("Should keep a dismissed occurrence closed until that @ itself changes", () => {
    const store = sessionMentionTriggerLogic.createStore({ workspaceId: "" });
    const active = () => activeMentionTrigger(store.getSnapshot().context);
    const observe = (text: string, cursor: number) =>
      store.trigger.observed({ trigger: detectMentionTrigger(text, cursor), text });

    observe("The @Ov", 7);
    expect(active()).toEqual({ offset: 4, query: "Ov" });
    store.trigger.dismissed();
    expect(active()).toBeNull();

    // Typing after the dismissed `@` keeps it closed.
    observe("The @Override", 13);
    expect(active()).toBeNull();
    // The caret leaves the token and comes back into it.
    observe("The @Override annotation", 24);
    expect(active()).toBeNull();
    observe("The @Override annotation", 13);
    expect(active()).toBeNull();
    // A new `@` after whitespace opens the picker.
    observe("The @Override annotation @", 26);
    expect(active()).toEqual({ offset: 25, query: "" });
    // Text inserted before the dismissed `@` moves it; it stays closed.
    observe("See The @Override annotation @", 17);
    expect(active()).toBeNull();
    // Deleting that `@` ends the dismissal: a freshly typed `@` there opens.
    observe("See The Override annotation @", 8);
    observe("See The @Override annotation @", 17);
    expect(active()).toEqual({ offset: 8, query: "Override" });
  });

  it("Should carry dismissed offsets across an edit by its common prefix and suffix", () => {
    expect(remapDismissedOffsets([4], "The @Ov", "The @Override")).toEqual([4]);
    expect(remapDismissedOffsets([4], "The @Ov", "See The @Ov")).toEqual([8]);
    expect(remapDismissedOffsets([4], "The @Ov", "The Ov")).toEqual([]);
    expect(remapDismissedOffsets([0, 3], "@a @b", "")).toEqual([]);
    const unchanged = [3];
    expect(remapDismissedOffsets(unchanged, "ab @c", "ab @c")).toBe(unchanged);
  });

  it("Should persist picks as newest-first recents per workspace", () => {
    const store = sessionMentionTriggerLogic.createStore({ workspaceId: "ws-1" });
    store.trigger.picked({ recent: { path: "a.ts", kind: "file" } });
    store.trigger.picked({ recent: { path: "web/", kind: "folder" } });
    store.trigger.picked({ recent: { path: "a.ts", kind: "file" } });
    expect(store.getSnapshot().context.recents).toEqual([
      { path: "a.ts", kind: "file" },
      { path: "web/", kind: "folder" },
    ]);
    expect(readSessionMentionRecents("ws-1")).toEqual(store.getSnapshot().context.recents);
    expect(readSessionMentionRecents("ws-2")).toEqual([]);
  });
});

describe("session mention recents", () => {
  it("Should cap at eight and drop corrupt storage", () => {
    let recents = withSessionMentionRecent([], { path: "0.ts", kind: "file" });
    for (let index = 1; index < 10; index++) {
      recents = withSessionMentionRecent(recents, { path: `${index}.ts`, kind: "file" });
    }
    expect(recents).toHaveLength(8);
    expect(recents[0]!.path).toBe("9.ts");

    writeSessionMentionRecents("ws-cap", recents);
    expect(readSessionMentionRecents("ws-cap")).toEqual(recents);

    window.localStorage.setItem(sessionMentionRecentsKey("ws-bad"), "{not json");
    expect(readSessionMentionRecents("ws-bad")).toEqual([]);
    window.localStorage.setItem(
      sessionMentionRecentsKey("ws-mixed"),
      JSON.stringify([{ path: "ok.ts", kind: "file" }, { path: 3 }, { path: "x", kind: "agent" }])
    );
    expect(readSessionMentionRecents("ws-mixed")).toEqual([{ path: "ok.ts", kind: "file" }]);
  });
});
