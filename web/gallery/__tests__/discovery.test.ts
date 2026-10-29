// @vitest-environment node
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { findPrototypes } from "../node/discovery";
import { findSpecs, watchSpecs, type SpecCatalog } from "../node/specs";

const roots: string[] = [];

function project() {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "compozy-gallery-")));
  roots.push(root);
  const web = join(root, "web");
  mkdirSync(join(web, "src"), { recursive: true });
  const write = (file: string, body: string) => {
    const path = join(root, file);
    mkdirSync(join(path, ".."), { recursive: true });
    writeFileSync(path, body);
  };
  return { root, web, write };
}

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

const meta = (extra = "") =>
  `export const meta = definePrototype({ title: "Board", ${extra} states: [{ id: "ready", label: "Ready" }] });`;
const noSpecs: SpecCatalog = { available: false, specs: [] };
const component = "export default function Prototype() { return null; }";

function codes(app: ReturnType<typeof project>, catalog: SpecCatalog = noSpecs) {
  return findPrototypes(app.web, app.root, catalog).problems.map(problem => problem.code);
}

describe("prototype discovery", () => {
  it("reads literal metadata without executing the module", () => {
    const app = project();
    app.write(
      "web/src/systems/session/board.proto.tsx",
      `${meta('surface: "S5",')}\n${component}\nthrow new Error("must not execute");`
    );
    const result = findPrototypes(app.web, app.root, noSpecs);
    expect(result.problems).toEqual([]);
    expect(result.entries).toEqual([
      {
        slug: "board",
        kind: "component",
        file: "web/src/systems/session/board.proto.tsx",
        meta: { title: "Board", surface: "S5", states: [{ id: "ready", label: "Ready" }] },
      },
    ]);
  });

  it("names drafts after their folder and lets them use markup", () => {
    const app = project();
    app.write(
      "web/src/drafts/new-board/index.proto.tsx",
      `${meta()}\nexport default () => <div />;`
    );
    const result = findPrototypes(app.web, app.root, noSpecs);
    expect(result.problems).toEqual([]);
    expect(result.entries[0]).toMatchObject({ slug: "new-board", kind: "draft" });
  });

  it("reports duplicate slugs with both files and intrinsic markup in component prototypes", () => {
    const app = project();
    app.write("web/src/drafts/board/index.proto.tsx", `${meta()}\nexport default () => <div />;`);
    app.write(
      "web/src/systems/session/board.proto.tsx",
      `${meta()}\nexport default () => <div />;`
    );
    const result = findPrototypes(app.web, app.root, noSpecs);
    expect(result.problems.map(problem => problem.code)).toEqual(["duplicate_slug", "markup"]);
    expect(result.problems[0]?.message).toContain("web/src/drafts/board/index.proto.tsx");
    expect(result.problems[0]?.message).toContain("web/src/systems/session/board.proto.tsx");
    expect(result.problems[1]?.message).toContain("board.proto.tsx:2:");
  });

  it("rejects invalid and reserved slugs", () => {
    const app = project();
    app.write("web/src/systems/session/Board.proto.tsx", `${meta()}\n${component}`);
    app.write("web/src/systems/session/flow-board.proto.tsx", `${meta()}\n${component}`);
    expect(codes(app)).toEqual(["invalid_slug", "invalid_slug"]);
  });

  it.each([
    ["a computed value", "export const meta = definePrototype(loadMeta());"],
    [
      "a duplicated key",
      'export const meta = definePrototype({ title: "A", title: "B", states: [] });',
    ],
    [
      "duplicated state ids",
      'export const meta = definePrototype({ title: "A", states: [{ id: "x", label: "X" }, { id: "x", label: "Y" }] });',
    ],
  ])("rejects metadata with %s and files outside the supported roots", (_label, source) => {
    const app = project();
    app.write("web/src/routes/board.proto.tsx", `${source}\n${component}`);
    expect(codes(app)).toEqual(["outside_root", "missing_meta"]);
  });

  it("rejects unknown metadata fields, a missing default export and reads parenthesized values", () => {
    const app = project();
    app.write(
      "web/src/systems/session/board.proto.tsx",
      `export const meta = definePrototype({ title: ("Board"), states: [{ id: "ready", label: "Ready" }] });\nexport { Board as default };`
    );
    expect(codes(app)).toEqual([]);
    app.write("web/src/systems/session/card.proto.tsx", meta());
    app.write(
      "web/src/systems/session/row.proto.tsx",
      `export const meta = definePrototype({ title: "Row", owner: "x", states: [{ id: "a", label: "A" }] });\n${component}`
    );
    expect(codes(app)).toEqual(["missing_default", "missing_meta"]);
  });

  it("validates spec ids against the catalog only when the catalog exists", () => {
    const app = project();
    app.write(
      "web/src/systems/session/board.proto.tsx",
      `${meta('spec: "sessions-kanban",')}\n${component}`
    );
    expect(codes(app)).toEqual([]);
    const known = { available: true, specs: [{ id: "sessions-kanban", title: "Sessions Kanban" }] };
    expect(codes(app, known)).toEqual([]);
    expect(codes(app, { available: true, specs: [] })).toEqual(["unknown_spec"]);
  });
});

describe("spec derivation", () => {
  it("lists .compozy/tasks directories with their _spec.md title", () => {
    const app = project();
    expect(findSpecs(app.root)).toEqual({ available: false, specs: [] });
    app.write(
      ".compozy/tasks/sessions-kanban/_spec.md",
      "# Sessions Kanban — Specification\n\nBody"
    );
    app.write(".compozy/tasks/agent-plugins/notes.md", "no spec yet");
    app.write(".compozy/tasks/_archived/old/_spec.md", "# Old");
    app.write(".compozy/tasks/.dispatch/state.json", "{}");
    expect(findSpecs(app.root)).toEqual({
      available: true,
      specs: [
        { id: "agent-plugins", title: "agent-plugins" },
        { id: "sessions-kanban", title: "Sessions Kanban" },
      ],
    });
  });

  it("notices a retitled spec and a new spec folder while watching", async () => {
    const app = project();
    app.write(".compozy/tasks/sessions-kanban/_spec.md", "# Sessions Kanban");
    let changes = 0;
    const stop = watchSpecs(app.root, () => {
      changes += 1;
    });
    const settle = async (before: number) => {
      for (let tries = 0; tries < 100 && changes === before; tries += 1) {
        await new Promise(resolve => setTimeout(resolve, 20));
      }
      return changes > before;
    };
    try {
      app.write(".compozy/tasks/sessions-kanban/_spec.md", "# Sessions Board");
      expect(await settle(0)).toBe(true);
      const seen = changes;
      mkdirSync(join(app.root, ".compozy/tasks/agent-plugins"));
      expect(await settle(seen)).toBe(true);
    } finally {
      stop();
    }
  });
});
