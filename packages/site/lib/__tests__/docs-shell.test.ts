import type { Folder, Item, Root, Separator } from "fumadocs-core/page-tree";
import { describe, expect, it, vi } from "vitest";

// `buildDocsShell` takes its tree and page facts as arguments; only `getDocsShell` reads the source.
vi.mock("@/lib/source", () => ({ docsSource: {} }));

import { buildDocsShell, countNavPages, neighboursOf, topFolderOf } from "@/lib/docs-shell";

const page = (url: string, name: string): Item => ({ type: "page", url, name });
const separator = (name: string): Separator => ({ type: "separator", name });
const folder = (name: string, children: Folder["children"], index?: Item): Folder => ({
  type: "folder",
  $id: name.toLowerCase(),
  name,
  index,
  children,
});

// The shape `overviewPageTransformer` leaves: a folder's landing page is also its first child.
const sessionsIndex = page("/docs/sessions", "Sessions");
const cliIndex = page("/docs/cli", "CLI Reference");
const tree: Root = {
  name: "Docs",
  children: [
    page("/docs", "Overview"),
    page("/docs/how-to-use-these-docs", "How to use these docs"),
    separator("Start"),
    folder(
      "Sessions",
      [
        page("/docs/sessions", "Overview"),
        page("/docs/sessions/lifecycle", "Lifecycle"),
        folder("Advanced", [page("/docs/sessions/advanced/fork", "Fork")]),
      ],
      sessionsIndex
    ),
    page("/docs/quickstart", "Quickstart"),
    separator("Reference"),
    folder("CLI", [page("/docs/cli", "Overview"), page("/docs/cli/session", "session")], cliIndex),
  ],
};

const shell = buildDocsShell(tree, url => ({
  title: `Title of ${url}`,
  description: `About ${url}`,
}));

describe("docs shell", () => {
  it("reads every page once, in sidebar order, the docs home first", () => {
    expect(shell.pages.map(place => place.url)).toEqual([
      "/docs",
      "/docs/how-to-use-these-docs",
      "/docs/sessions",
      "/docs/sessions/lifecycle",
      "/docs/sessions/advanced/fork",
      "/docs/quickstart",
      "/docs/cli",
      "/docs/cli/session",
    ]);
    expect(shell.byUrl.get("/docs/quickstart")?.title).toBe("Title of /docs/quickstart");
  });

  it("drops the landing page the overview transformer repeats as a folder's first child", () => {
    const [sessions] = shell.sections[0]?.nodes ?? [];
    expect(sessions?.url).toBe("/docs/sessions");
    expect(sessions?.children?.map(node => node.name)).toEqual(["Lifecycle", "Advanced"]);
    expect(sessions && countNavPages(sessions)).toBe(3);
  });

  it("counts each section's distinct pages and the pages ahead of the first section", () => {
    expect(shell.sections.map(({ id, label, count }) => ({ id, label, count }))).toEqual([
      { id: "start", label: "Start", count: 4 },
      { id: "reference", label: "Reference", count: 2 },
    ]);
    expect(shell.total).toBe(8);
  });

  it("points each tab at its section's first page and the path heads it owns", () => {
    expect(shell.tabs).toEqual([
      {
        id: "start",
        label: "Start",
        count: 4,
        href: "/docs/sessions",
        heads: ["sessions", "quickstart"],
      },
      { id: "reference", label: "Reference", count: 2, href: "/docs/cli", heads: ["cli"] },
    ]);
  });

  it("places each page in its section under the folders above it", () => {
    const fork = shell.byUrl.get("/docs/sessions/advanced/fork");
    expect(fork?.section?.id).toBe("start");
    expect(fork?.trail.map(node => node.name)).toEqual(["Sessions", "Advanced"]);
    expect(fork && topFolderOf(fork)?.name).toBe("Sessions");
    const landing = shell.byUrl.get("/docs/sessions");
    expect(landing?.trail).toEqual([]);
    expect(landing && topFolderOf(landing)?.name).toBe("Sessions");
    expect(shell.byUrl.get("/docs/how-to-use-these-docs")?.section).toBeNull();
  });

  it("pages from one neighbour to the next across folders and sections", () => {
    const names = (url: string) => {
      const { previous, next } = neighboursOf(shell, url);
      return [previous?.url ?? null, next?.url ?? null];
    };
    expect(names("/docs")).toEqual([null, "/docs/how-to-use-these-docs"]);
    expect(names("/docs/quickstart/")).toEqual(["/docs/sessions/advanced/fork", "/docs/cli"]);
    expect(names("/docs/cli/session")).toEqual(["/docs/cli", null]);
    expect(names("/docs/missing")).toEqual([null, null]);
  });
});
