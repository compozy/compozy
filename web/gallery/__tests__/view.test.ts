import { describe, expect, it } from "vitest";

import {
  canonicalParams,
  frameSrc,
  readView,
  resolveSelection,
  selectionHref,
  type GalleryView,
} from "../shell/view";

const catalog = {
  prototypeExists: (slug: string) => slug === "session-board-card",
  specExists: (spec: string) => spec === "sessions-kanban",
  flowNodes: (flow: string) => (flow === "board-to-pull-request" ? ["board", "card"] : null),
};

function resolve(search: string) {
  return resolveSelection(readView(search), catalog);
}

describe("gallery URL view", () => {
  it("reads the English parameters and drops out-of-range widths", () => {
    expect(readView("?p=card&state=merged&theme=light&w=390&compare=themes")).toEqual({
      spec: null,
      slug: "card",
      state: "merged",
      theme: "light",
      width: 390,
      compare: true,
      comments: false,
      comment: null,
      flow: null,
      node: null,
    } satisfies GalleryView);
    expect(readView("?p=card&comments=1&comment=card-2")).toMatchObject({
      comments: true,
      comment: "card-2",
    });
    expect(readView("?w=100&theme=sepia")).toMatchObject({ width: "fit", theme: null });
  });

  it("resolves a prototype over a spec and falls back to the next level up", () => {
    expect(resolve("?p=session-board-card&spec=sessions-kanban")).toEqual({
      kind: "prototype",
      slug: "session-board-card",
    });
    expect(resolve("?p=missing&spec=sessions-kanban")).toEqual({
      kind: "spec",
      spec: "sessions-kanban",
    });
    expect(resolve("?spec=missing")).toEqual({ kind: "index" });
  });

  it("builds selection links that clear the other selection and keep the theme", () => {
    const search = "?p=session-board-card&state=merged&w=390&theme=light";
    expect(selectionHref({ kind: "spec", spec: "sessions-kanban" }, search)).toBe(
      "/?theme=light&spec=sessions-kanban"
    );
    expect(selectionHref({ kind: "index" }, search)).toBe("/?theme=light");
    expect(
      selectionHref({ kind: "prototype", slug: "session-board-card", state: "current" }, search)
    ).toBe("/?p=session-board-card&state=current&w=390&theme=light");
  });

  it("canonicalizes parameters that do not name what is on screen", () => {
    const stale = "?spec=sessions-kanban&p=session-board-card&state=nope&theme=dark&w=9";
    expect(canonicalParams(stale, resolve(stale), "needs-you")).toEqual({
      spec: null,
      state: "needs-you",
      theme: null,
      w: null,
    });
    const index = "?state=x&compare=themes&theme=light";
    expect(canonicalParams(index, resolve(index), null)).toEqual({ state: null, compare: null });
    const compare = "?p=session-board-card&state=merged&compare=yes";
    expect(canonicalParams(compare, resolve(compare), "merged")).toEqual({ compare: null });
    const clean = "?p=session-board-card&state=merged&theme=light";
    expect(canonicalParams(clean, resolve(clean), "merged")).toBeNull();
  });

  it("keeps the comments panel across prototypes and drops the selected comment", () => {
    const search = "?p=session-board-card&comments=1&comment=session-board-card-3";
    expect(selectionHref({ kind: "prototype", slug: "session-list-row" }, search)).toBe(
      "/?p=session-list-row&comments=1"
    );
    expect(selectionHref({ kind: "spec", spec: "sessions-kanban" }, search)).toBe(
      "/?spec=sessions-kanban"
    );
    expect(canonicalParams(search, resolve(search), "ready")).toBeNull();
    const stale = "?p=session-board-card&comments=yes&comment=";
    expect(canonicalParams(stale, resolve(stale), "ready")).toEqual({
      comments: null,
      comment: null,
    });
  });

  it("points the canvas at the chrome-less standalone URL", () => {
    expect(frameSrc("session-board-card", { state: "merged", theme: null })).toBe(
      "/session-board-card?chrome=0&state=merged"
    );
    expect(frameSrc("session-board-card", { state: "merged", theme: "light" }, true)).toBe(
      "/session-board-card?theme=light&state=merged"
    );
  });

  it("resolves a flow over a prototype, and a missing flow unless a prototype is named", () => {
    expect(resolve("?flow=board-to-pull-request&node=card&p=session-board-card")).toEqual({
      kind: "flow",
      flow: "board-to-pull-request",
      node: "card",
    });
    expect(resolve("?flow=board-to-pull-request&node=gone")).toEqual({
      kind: "flow",
      flow: "board-to-pull-request",
      node: null,
    });
    expect(resolve("?flow=missing")).toEqual({ kind: "missing-flow", flow: "missing" });
    expect(resolve("?flow=missing&p=session-board-card")).toEqual({
      kind: "prototype",
      slug: "session-board-card",
    });
  });

  it("links flows and their nodes, keeping the comments panel and the theme", () => {
    const search = "?p=session-board-card&state=merged&w=390&comments=1&comment=x-1&theme=light";
    expect(
      selectionHref({ kind: "flow", flow: "board-to-pull-request", node: "card" }, search)
    ).toBe("/?comments=1&theme=light&flow=board-to-pull-request&node=card");
    const flow = "?flow=board-to-pull-request&node=card&comments=1";
    expect(selectionHref({ kind: "prototype", slug: "session-board-card" }, flow)).toBe(
      "/?comments=1&p=session-board-card"
    );
  });

  it("canonicalizes flow URLs: stale nodes and other selections drop, a missing flow stays", () => {
    const stale = "?flow=board-to-pull-request&node=gone&spec=sessions-kanban&w=390&comments=yes";
    expect(canonicalParams(stale, resolve(stale), null)).toEqual({
      spec: null,
      w: null,
      node: null,
      comments: null,
    });
    const clean = "?flow=board-to-pull-request&node=card&comments=1";
    expect(canonicalParams(clean, resolve(clean), null)).toBeNull();
    const missing = "?flow=missing&w=390";
    expect(canonicalParams(missing, resolve(missing), null)).toBeNull();
  });
});
