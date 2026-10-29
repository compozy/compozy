import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import ts from "typescript";

import type { PrototypeMeta } from "../contract.ts";
import { flowKeyPrefix } from "../flow.ts";
import { readLiteral } from "./literal.ts";
import type { SpecCatalog } from "./specs.ts";

/** `component` prototypes compose real views; `draft` prototypes may use free markup. */
export type PrototypeKind = "component" | "draft";
export type PrototypeEntry = {
  slug: string;
  kind: PrototypeKind;
  /** Repository-relative path. */
  file: string;
  meta: PrototypeMeta | null;
};
export type ProblemCode =
  | "duplicate_slug"
  | "invalid_slug"
  | "outside_root"
  | "missing_meta"
  | "unknown_spec"
  | "markup"
  | "api_client";
export type Problem = { slug: string | null; file: string; code: ProblemCode; message: string };

export const slugPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/;
export const maxSlugLength = 120;
/** Reserved for review keys that are not prototypes (journey flows' comments). */
export const reservedSlugPrefix = flowKeyPrefix;

const protoSuffix = ".proto.tsx";

function files(root: string): string[] {
  return readdirSync(root, { withFileTypes: true }).flatMap(entry => {
    const path = join(root, entry.name);
    if (entry.isDirectory()) return entry.name === "node_modules" ? [] : files(path);
    return entry.isFile() && entry.name.endsWith(protoSuffix) ? [path] : [];
  });
}

function isState(value: unknown): value is { id: string; label: string } {
  if (!value || typeof value !== "object") return false;
  const { id, label } = value as { id?: unknown; label?: unknown };
  return typeof id === "string" && id.length > 0 && typeof label === "string" && label.length > 0;
}

function optionalText(value: unknown) {
  return value === undefined || (typeof value === "string" && value.trim().length > 0);
}

/** The literal `meta` when it satisfies the contract, otherwise null. */
function validMeta(raw: unknown): PrototypeMeta | null {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null;
  const meta = raw as Record<string, unknown>;
  if (typeof meta.title !== "string" || meta.title.trim().length === 0) return null;
  if (!optionalText(meta.spec) || !optionalText(meta.surface) || !optionalText(meta.route)) {
    return null;
  }
  const states = meta.states;
  if (!Array.isArray(states) || states.length === 0 || !states.every(isState)) return null;
  if (new Set(states.map(state => state.id)).size !== states.length) return null;
  return raw as PrototypeMeta;
}

function metadata(source: ts.SourceFile): PrototypeMeta | null {
  for (const statement of source.statements) {
    if (
      !ts.isVariableStatement(statement) ||
      !statement.modifiers?.some(modifier => modifier.kind === ts.SyntaxKind.ExportKeyword)
    ) {
      continue;
    }
    for (const declaration of statement.declarationList.declarations) {
      const value = declaration.initializer;
      if (
        declaration.name.getText(source) !== "meta" ||
        !value ||
        !ts.isCallExpression(value) ||
        value.expression.getText(source) !== "definePrototype" ||
        !value.arguments[0]
      ) {
        continue;
      }
      try {
        return validMeta(readLiteral(value.arguments[0]));
      } catch {
        return null;
      }
    }
  }
  return null;
}

/** Intrinsic elements (`<div>`) in a component prototype, which must render views only. */
function markup(source: ts.SourceFile) {
  const found: { tag: string; line: number; column: number }[] = [];
  const visit = (node: ts.Node) => {
    if (
      (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
      /^[a-z]/.test(node.tagName.getText(source))
    ) {
      const at = source.getLineAndCharacterOfPosition(node.getStart(source));
      found.push({
        tag: node.tagName.getText(source),
        line: at.line + 1,
        column: at.character + 1,
      });
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  return found;
}

/** Slug of a prototype file: its stem, or the parent directory for `index.proto.tsx`. */
export function slugOf(local: string) {
  const parts = local.split("/");
  const name = parts.at(-1) ?? "";
  return name === `index${protoSuffix}` ? (parts.at(-2) ?? "") : name.slice(0, -protoSuffix.length);
}

/**
 * Finds every `*.proto.tsx` under `web/src` by reading the syntax tree only: modules never run.
 * Spec prototypes live beside their view in `src/systems` or `src/components`; exploratory drafts
 * in `src/drafts/<slug>/index.proto.tsx`.
 */
export function findPrototypes(webRoot: string, repoRoot: string, catalog: SpecCatalog) {
  const entries: PrototypeEntry[] = [];
  const problems: Problem[] = [];
  const specIds = new Set(catalog.specs.map(spec => spec.id));
  for (const path of files(join(webRoot, "src")).sort()) {
    const local = relative(webRoot, path);
    const file = relative(repoRoot, path);
    const slug = slugOf(local);
    const kind: PrototypeKind = local.startsWith("src/drafts/") ? "draft" : "component";
    const report = (code: ProblemCode, message: string) =>
      problems.push({ slug, file, code, message });
    if (!slugPattern.test(slug) || slug.length > maxSlugLength) {
      report("invalid_slug", `Invalid prototype slug "${slug}" (${file})`);
    } else if (slug.startsWith(reservedSlugPrefix)) {
      report("invalid_slug", `"${reservedSlugPrefix}" is reserved for flows: "${slug}" (${file})`);
    }
    if (
      !/^src\/(systems|components)\/.+\.proto\.tsx$/.test(local) &&
      !/^src\/drafts\/[^/]+\/index\.proto\.tsx$/.test(local)
    ) {
      report(
        "outside_root",
        `${file}: prototypes live in web/src/systems, web/src/components or web/src/drafts/<slug>/index.proto.tsx`
      );
    }
    const previous = entries.find(entry => entry.slug === slug);
    if (previous) {
      report("duplicate_slug", `Duplicate prototype slug "${slug}": ${previous.file} and ${file}`);
    }
    const source = ts.createSourceFile(
      file,
      readFileSync(path, "utf8"),
      ts.ScriptTarget.Latest,
      true,
      ts.ScriptKind.TSX
    );
    const meta = metadata(source);
    if (!meta) {
      report("missing_meta", `${slug}: missing or invalid literal "export const meta"`);
    } else if (meta.spec && catalog.available && !specIds.has(meta.spec)) {
      report(
        "unknown_spec",
        `${slug}: unknown spec "${meta.spec}" (no .compozy/tasks/${meta.spec}/)`
      );
    }
    if (kind === "component") {
      for (const element of markup(source)) {
        report(
          "markup",
          `Spec prototypes render components only: <${element.tag}> at ${file}:${element.line}:${element.column}. Move the markup into a view component, or explore in web/src/drafts.`
        );
      }
    }
    entries.push({ slug, kind, file, meta });
  }
  return { entries, problems };
}
