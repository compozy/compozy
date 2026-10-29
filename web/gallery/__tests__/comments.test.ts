import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  renameSync,
  rmSync,
  utimesSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { AttachmentStore, pendingLifetime } from "../node/attachments-store";
import { CommentError, CommentStore, keyOfId } from "../node/comments-store";
import { inspectImage } from "../node/image";
import { isTrustedWrite } from "../node/trusted-write";
import { describeTarget, locateTarget, selectorFor, sourceChain } from "../shell/comments/dom";
import {
  parseNewComment,
  type CommentFile,
  type ElementTarget,
  type NewComment,
} from "../shell/comments/model";

const directories: string[] = [];

afterEach(() => {
  for (const directory of directories.splice(0))
    rmSync(directory, { recursive: true, force: true });
  document.body.innerHTML = "";
});

function temporary() {
  const directory = mkdtempSync(join(tmpdir(), "compozy-comments-"));
  directories.push(directory);
  return directory;
}

const known = new Set(["session-board-card", "session-list-row"]);

function store(now = () => new Date("2026-10-29T12:00:00.000Z")) {
  const directory = temporary();
  return new CommentStore({
    directory,
    now,
    assertKey: key => {
      if (!known.has(key)) throw new CommentError(`Unknown prototype "${key}"`, 404);
    },
  });
}

const rect = { x: 10, y: 20, width: 100, height: 40 };

const target: ElementTarget = {
  kind: "element",
  selector: '[data-testid="card"]',
  testId: "card",
  tag: "article",
  text: "Fix login",
  source: "web/src/systems/session/components/session-board/session-board-card.tsx:42:5",
  sourceChain: ["web/src/systems/session/components/session-board/session-board-card.tsx:42:5"],
  rect,
  rectNormalized: { x: 0.0123456, y: 0.1, width: 0.2, height: 0.05 },
  offset: null,
};

const input = (body = "Tighten the title spacing"): NewComment => ({
  body,
  target,
  context: {
    commit: null,
    state: "waiting",
    theme: "dark",
    viewport: { width: 1440, height: 900 },
  },
});

/** A structurally complete PNG: IHDR with the given size, one empty IDAT and IEND (CRCs zeroed). */
function png(width = 2, height = 3) {
  const bytes = new Uint8Array(57);
  bytes.set([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 0x49, 0x48, 0x44, 0x52]);
  const view = new DataView(bytes.buffer);
  view.setUint32(16, width);
  view.setUint32(20, height);
  bytes.set([0, 0, 0, 0, 0x49, 0x44, 0x41, 0x54], 33);
  bytes.set([0, 0, 0, 0, 0x49, 0x45, 0x4e, 0x44], 45);
  return bytes;
}

describe("comment store", () => {
  it("creates, replies, resolves with a note, reopens, edits and deletes", () => {
    const comments = store();
    const created = comments.add("session-board-card", input());
    expect(created).toMatchObject({ id: "session-board-card-1", status: "open", resolvedAt: null });
    expect(created.thread).toEqual([
      expect.objectContaining({ author: "reviewer", body: "Tighten the title spacing" }),
    ]);

    comments.reply(created.id, "agent", "Which state?");
    const resolved = comments.setStatus(created.id, "resolved", { author: "agent", body: "Done" });
    expect(resolved.status).toBe("resolved");
    expect(resolved.resolvedAt).toBe("2026-10-29T12:00:00.000Z");
    expect(resolved.thread.map(message => message.body)).toEqual([
      "Tighten the title spacing",
      "Which state?",
      "Done",
    ]);

    const reopened = comments.setStatus(created.id, "open");
    expect(reopened).toMatchObject({ status: "open", resolvedAt: null });
    expect(reopened.thread).toHaveLength(3);

    const reply = reopened.thread[1]!;
    const edited = comments.editMessage(created.id, reply.id, { body: "Which state, exactly?" });
    expect(edited.thread[1]).toMatchObject({
      id: reply.id,
      body: "Which state, exactly?",
      editedAt: "2026-10-29T12:00:00.000Z",
    });
    expect(edited.thread[0]?.editedAt).toBeNull();

    expect(comments.counts()).toEqual({ "session-board-card": { open: 1, resolved: 0 } });
    comments.remove(created.id);
    expect(comments.read("session-board-card")).toEqual([]);
    expect(comments.counts()).toEqual({});
  });

  it("writes a canonical version 2 file with rounded rects and no served URLs", () => {
    const comments = store();
    comments.add("session-board-card", input());
    const file = JSON.parse(
      readFileSync(comments.path("session-board-card"), "utf8")
    ) as CommentFile;
    expect(file.version).toBe(2);
    expect(file.slug).toBe("session-board-card");
    expect(file.comments[0]?.target).toMatchObject({ rectNormalized: { x: 0.0123 } });
    expect(Object.keys(file.comments[0]!)).toEqual([
      "id",
      "slug",
      "status",
      "createdAt",
      "updatedAt",
      "resolvedAt",
      "context",
      "target",
      "thread",
    ]);
  });

  it("numbers ids per key and finds the key of an id", () => {
    const comments = store();
    const ids = [1, 2, 3].map(() => comments.add("session-board-card", input()).id);
    expect(ids).toEqual(["session-board-card-1", "session-board-card-2", "session-board-card-3"]);
    comments.remove("session-board-card-2");
    expect(comments.add("session-board-card", input()).id).toBe("session-board-card-4");
    // Deleted ids stay retired: neither the highest nor a full wipe hands a number out again.
    comments.remove("session-board-card-4");
    expect(comments.add("session-board-card", input()).id).toBe("session-board-card-5");
    for (const id of ["1", "3", "5"]) comments.remove(`session-board-card-${id}`);
    expect(comments.add("session-board-card", input()).id).toBe("session-board-card-6");
    expect(comments.add("session-list-row", input()).id).toBe("session-list-row-1");
    expect(keyOfId("session-list-row-12")).toBe("session-list-row");
    expect(() => keyOfId("session-list-row")).toThrow(CommentError);
  });

  it("serializes changes through a per-key lock and reclaims a dead writer's lock", () => {
    const comments = store();
    const lock = join(comments.directory, "session-board-card", ".comments.lock");
    mkdirSync(lock, { recursive: true });
    const stale = new Date(Date.now() - 60_000);
    utimesSync(lock, stale, stale);
    const created = comments.add("session-board-card", input());
    expect(created.id).toBe("session-board-card-1");
    expect(existsSync(lock)).toBe(false);
    comments.reply(created.id, "agent", "On it");
    const folder = readdirSync(join(comments.directory, "session-board-card"));
    expect(folder.filter(name => name !== "comments.json" && name !== "attachments")).toEqual([]);
  });

  it("accepts writes only from the gallery's own origin or non-browser callers", () => {
    const host = "localhost:5180";
    expect(isTrustedWrite({ host })).toBe(true);
    expect(isTrustedWrite({ host, origin: "http://localhost:5180" })).toBe(true);
    expect(
      isTrustedWrite({ host, origin: "http://localhost:5180", "sec-fetch-site": "same-origin" })
    ).toBe(true);
    expect(isTrustedWrite({ host, origin: "https://evil.example" })).toBe(false);
    expect(isTrustedWrite({ host, origin: "null" })).toBe(false);
    expect(isTrustedWrite({ host, "sec-fetch-site": "cross-site" })).toBe(false);
    expect(isTrustedWrite({ host, "sec-fetch-site": "same-site" })).toBe(false);
  });

  it("rejects unknown keys, missing comments and empty bodies", () => {
    const comments = store();
    expect(() => comments.read("missing")).toThrow('Unknown prototype "missing"');
    expect(() => comments.get("session-board-card-9")).toThrow("not found");
    expect(() => comments.add("session-board-card", input("   "))).toThrow("Invalid message body");
    const created = comments.add("session-board-card", input());
    expect(() => comments.reply(created.id, "agent", "")).toThrow("Invalid message body");
    expect(() => comments.editMessage(created.id, created.thread[0]!.id, {})).toThrow(
      "Empty message update"
    );
  });

  it("validates new comments from the API", () => {
    expect(parseNewComment(input())).toEqual(input());
    expect(parseNewComment({ ...input(), body: "" })).toBeNull();
    expect(
      parseNewComment({ ...input(), context: { ...input().context, commit: "HEAD" } })
    ).toBeNull();
    expect(parseNewComment({ ...input(), target: { ...target, rect: { x: 1 } } })).toBeNull();
    expect(parseNewComment({ ...input(), attachments: ["../etc/passwd"] })).toBeNull();
    const node = { kind: "node", node: "review", label: "Review step" };
    expect(parseNewComment({ ...input(), target: node })?.target).toEqual(node);
  });
});

describe("comment attachments", () => {
  it("reads image type and size from magic bytes, not names", () => {
    expect(inspectImage(png(640, 480))).toEqual({
      type: "image/png",
      width: 640,
      height: 480,
      size: 57,
    });
    const gif = new Uint8Array([0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 16, 0, 8, 0]);
    expect(inspectImage(gif)).toMatchObject({ type: "image/gif", width: 16, height: 8 });
    expect(() => inspectImage(new TextEncoder().encode("<svg></svg>"))).toThrow(
      "unsupported_media_type"
    );
    expect(() => inspectImage(png(0, 10))).toThrow("unsupported_media_type");
    // Truncated before IEND: the header alone is not an image.
    expect(() => inspectImage(png(640, 480).subarray(0, 33))).toThrow("unsupported_media_type");
    expect(() => inspectImage(new Uint8Array(5 * 1024 * 1024 + 1))).toThrow("attachment_too_large");
  });

  it("attaches pending uploads to a message and caps a message at six images", () => {
    const comments = store();
    const files = comments.attachments("session-board-card");
    const first = files.upload("screen.png", png());
    const created = comments.add("session-board-card", { ...input(), attachments: [first.id] });
    expect(created.thread[0]?.attachments).toEqual([first]);
    expect(files.read(first.id, comments.read("session-board-card")).bytes).toHaveLength(57);
    expect(readdirSync(files.pending)).toEqual([]);

    const more = Array.from({ length: 6 }, () => files.upload("more.png", png()));
    expect(() =>
      comments.editMessage(created.id, created.thread[0]!.id, {
        addAttachments: more.map(image => image.id),
      })
    ).toThrow("attachment_limit");

    const detached = comments.editMessage(created.id, created.thread[0]!.id, {
      removeAttachments: [first.id],
    });
    expect(detached.thread[0]?.attachments).toEqual([]);
    expect(existsSync(join(files.directory, `${first.id}.png`))).toBe(false);
  });

  it("rejects non-images and expires unclaimed uploads after a day", () => {
    let now = Date.parse("2026-10-29T12:00:00.000Z");
    const files = new AttachmentStore(temporary(), () => now);
    expect(() => files.upload("notes.png", new TextEncoder().encode("hello"))).toThrow(
      expect.objectContaining({ status: 415 })
    );
    const image = files.upload("screen.png", png());
    now += pendingLifetime + 1;
    expect(() => files.read(image.id, [])).toThrow("not_found");
    files.upload("next.png", png());
    expect(readdirSync(files.pending).filter(name => name.startsWith(image.id))).toEqual([]);
  });

  it("makes an image claimable again after a commit died before the comments write", () => {
    const comments = store();
    const files = comments.attachments("session-board-card");
    const image = files.upload("screen.png", png());
    // The writer moved the file into place and died before persisting the message.
    expect(() =>
      files.commit([], [image.id], [], () => {
        throw new Error("killed");
      })
    ).toThrow("killed");
    renameSync(join(files.pending, `${image.id}.png`), join(files.directory, `${image.id}.png`));
    writeFileSync(join(files.pending, "broken.json"), "{");
    const created = comments.add("session-board-card", { ...input(), attachments: [image.id] });
    expect(created.thread[0]?.attachments).toEqual([image]);
    expect(readdirSync(files.pending)).toEqual(["broken.json"]);
  });
});

describe("comment DOM targets", () => {
  function render(html: string) {
    document.body.innerHTML = html;
  }

  function box(element: Element, value: { x: number; y: number; width: number; height: number }) {
    element.getBoundingClientRect = () =>
      ({
        ...value,
        left: value.x,
        top: value.y,
        right: value.x + value.width,
        bottom: value.y + value.height,
        toJSON: () => value,
      }) as DOMRect;
  }

  it("builds the shortest unique selector, anchored on test ids", () => {
    render(
      `<main><section data-testid="board"><p>One</p><p>Two</p></section><section><p>A</p><p>B</p></section></main>`
    );
    const second = document.querySelectorAll("section p")[1]!;
    expect(selectorFor(second)).toBe('[data-testid="board"] > p:nth-of-type(2)');
    expect(document.querySelector(selectorFor(second))).toBe(second);
  });

  it("keeps the sibling position of repeated test ids", () => {
    render(
      `<main><div data-testid="column"><article data-testid="card">A</article><article data-testid="card">B</article></div></main>`
    );
    const second = document.querySelectorAll("article")[1]!;
    expect(document.querySelector(selectorFor(second))).toBe(second);
  });

  it("collects source tags nearest first and describes a click and a region", () => {
    render(
      `<div data-proto-src="web/src/a.proto.tsx:5:3"><article data-testid="card" data-proto-src="web/src/card.tsx:10:5"><span>Fix login</span></article></div>`
    );
    const span = document.querySelector("span")!;
    expect(sourceChain(span)).toEqual(["web/src/card.tsx:10:5", "web/src/a.proto.tsx:5:3"]);

    const article = document.querySelector("article")!;
    box(article, { x: 100, y: 50, width: 200, height: 100 });
    const canvas = { width: 1000, height: 500 };
    const click = describeTarget(article, null, canvas);
    expect(click).toMatchObject({
      kind: "element",
      testId: "card",
      tag: "article",
      text: "Fix login",
      source: "web/src/card.tsx:10:5",
      rectNormalized: { x: 0.1, y: 0.1, width: 0.2, height: 0.2 },
      offset: null,
    });

    document.elementFromPoint = () => span;
    const region = describeTarget(article, { x: 150, y: 75, width: 100, height: 50 }, canvas);
    expect(region.kind).toBe("region");
    expect(region.offset).toEqual({ x: 0.25, y: 0.25, width: 0.5, height: 0.5 });
  });

  it("locates a stored target on its anchor, else on the recorded rect", () => {
    render(`<article data-testid="card" data-proto-src="web/src/card.tsx:10:5"></article>`);
    const article = document.querySelector("article")!;
    box(article, { x: 200, y: 100, width: 200, height: 100 });
    const canvas = { width: 1000, height: 500 };
    const stored = { ...target, offset: { x: 0.5, y: 0, width: 0.5, height: 1 } };
    expect(locateTarget(document, stored, canvas)).toEqual({
      rect: { x: 300, y: 100, width: 100, height: 100 },
      anchored: true,
    });
    render(`<p>moved</p>`);
    expect(locateTarget(document, stored, canvas)).toEqual({
      rect: { x: 12.3456, y: 50, width: 200, height: 25 },
      anchored: false,
    });
  });
});
