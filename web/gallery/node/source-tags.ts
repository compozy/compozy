/**
 * Gallery transform that stamps `data-proto-src="web/src/...:<line>:<column>"` on JSX elements of
 * web views, prototypes and drafts, so a review comment can map a clicked element back to the JSX
 * that rendered it. React 19 dropped `_debugSource`, hence the attribute.
 *
 * The attribute goes first, so a component that spreads its props onto its root lets the call
 * site win. Elements that cannot take DOM attributes (fragments, providers, Suspense…) are
 * skipped. Text is only inserted, never moved, so line numbers stay exact.
 */
import { relative } from "node:path";
import ts from "typescript";
import type { Plugin } from "vite";

export const sourceAttribute = "data-proto-src";

const skippedNames =
  /(^|\.)(Fragment|Suspense|StrictMode|Activity|Profiler|\w*Provider|\w*Context)$/;

function taggable(tag: ts.JsxTagNameExpression) {
  if (ts.isJsxNamespacedName(tag)) return false;
  return !skippedNames.test(tag.getText());
}

function tagged(attributes: ts.JsxAttributes) {
  return attributes.properties.some(
    property => ts.isJsxAttribute(property) && property.name.getText() === sourceAttribute
  );
}

/** Returns the code with source attributes, or null when there is no JSX to tag. */
export function tagJsx(code: string, file: string): string | null {
  const source = ts.createSourceFile(file, code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const insertions: { at: number; text: string }[] = [];
  const visit = (node: ts.Node) => {
    if (
      (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
      taggable(node.tagName) &&
      !tagged(node.attributes)
    ) {
      const start = source.getLineAndCharacterOfPosition(node.getStart(source));
      insertions.push({
        at: node.attributes.pos,
        text: ` ${sourceAttribute}="${file}:${start.line + 1}:${start.character + 1}"`,
      });
    }
    ts.forEachChild(node, visit);
  };
  visit(source);
  if (insertions.length === 0) return null;
  let next = code;
  for (const { at, text } of [...insertions].sort((a, b) => b.at - a.at)) {
    next = next.slice(0, at) + text + next.slice(at);
  }
  return next;
}

export function sourceTags(repoRoot: string): Plugin {
  return {
    name: "compozy:prototype-source-tags",
    // Before the React Compiler (also `pre`), so it sees the authored attributes.
    enforce: "pre",
    transform: {
      filter: { id: /[\\/]web[\\/]src[\\/].+\.tsx$/ },
      handler(code, id) {
        const file = relative(repoRoot, id.split("?")[0] ?? id);
        if (!file.startsWith("web/src/")) return null;
        const next = tagJsx(code, file);
        return next === null ? null : { code: next, map: null };
      },
    },
  };
}
