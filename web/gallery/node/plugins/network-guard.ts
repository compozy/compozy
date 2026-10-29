import { realpathSync } from "node:fs";
import { relative } from "node:path";
import ts from "typescript";
import type { Plugin } from "vite";

import {
  forbiddenChain,
  isPrototypeModule,
  networkModuleMatcher,
  networkPackageSpecifier,
  violationMessage,
} from "../network-graph.ts";

const clean = (id: string) => id.split("?")[0] ?? id;

/**
 * Runtime import specifiers of emitted JavaScript (type-only imports are already erased), and
 * whether a dynamic `import()` computes its specifier, which no graph check can follow.
 */
function specifiersOf(code: string, file: string) {
  const syntax = ts.createSourceFile(file, code, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const specifiers = new Set<string>();
  let computed = false;
  const visit = (node: ts.Node) => {
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier &&
      ts.isStringLiteralLike(node.moduleSpecifier)
    ) {
      specifiers.add(node.moduleSpecifier.text);
    }
    if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      node.arguments[0]
    ) {
      if (ts.isStringLiteralLike(node.arguments[0])) specifiers.add(node.arguments[0].text);
      else computed = true;
    }
    ts.forEachChild(node, visit);
  };
  visit(syntax);
  return { specifiers, computed };
}

/**
 * Import-graph guard: a prototype or draft that reaches the daemon network layer fails the
 * gallery build (`proto check`) and shows an error in `proto dev`, including after edits made
 * while the server runs. Builds read the resolved module graph; dev tracks transformed modules.
 */
export function networkGuard(webRootInput: string): Plugin {
  const webRoot = realpathSync(webRootInput);
  const forbidden = networkModuleMatcher(webRoot);
  const imports = new Map<string, Set<string>>();
  const parents = new Map<string, Set<string>>();
  // Repository modules with an `import()` the graph cannot follow; reaching one counts as a
  // violation, like reaching the network layer. Third-party packages are out of scope.
  const computedImports = new Set<string>();
  const sourceRoot = `${webRoot}/src/`;
  let building = false;

  const check = (id: string) => {
    const chain = forbiddenChain(
      id,
      imports,
      module => forbidden(module) || computedImports.has(module)
    );
    if (!chain) return undefined;
    const last = chain.at(-1)!;
    if (!computedImports.has(last)) return violationMessage(chain, webRoot);
    const shown = chain.map(module => relative(webRoot, module)).join(" → ");
    return `api_client: ${relative(webRoot, last)} has an import() with a computed specifier, which cannot be checked against the network layer (${shown}); import a literal path.`;
  };

  return {
    name: "compozy:prototype-network-guard",
    enforce: "post",
    configResolved(config) {
      building = config.command === "build";
    },
    buildStart() {
      imports.clear();
      computedImports.clear();
      parents.clear();
    },
    buildEnd(error) {
      if (!building || error) return;
      for (const id of this.getModuleIds()) {
        const module = this.getModuleInfo(id);
        if (!module) continue;
        imports.set(
          clean(id),
          new Set([...module.importedIds, ...module.dynamicallyImportedIds].map(clean))
        );
      }
      for (const id of imports.keys()) {
        if (!isPrototypeModule(id)) continue;
        const message = check(id);
        if (message) this.error(message);
      }
    },
    async transform(code, source) {
      const from = clean(source);
      if (!/\.[cm]?[jt]sx?$/.test(from) || from.includes("\0")) return;
      const { specifiers, computed } = specifiersOf(code, from);
      if (computed && from.startsWith(sourceRoot)) computedImports.add(from);
      else computedImports.delete(from);
      if (building) return;
      const resolved = await Promise.all(
        [...specifiers].map(specifier => this.resolve(specifier, source))
      );
      const dependencies = new Set([
        ...resolved.flatMap(result => (result && !result.external ? [clean(result.id)] : [])),
        ...[...specifiers].filter(specifier => networkPackageSpecifier.test(specifier)),
      ]);
      // Replace outgoing edges so dependencies removed by an edit stop counting.
      for (const previous of imports.get(from) ?? []) parents.get(previous)?.delete(from);
      imports.set(from, dependencies);
      for (const to of dependencies) {
        const importers = parents.get(to) ?? new Set<string>();
        importers.add(from);
        parents.set(to, importers);
      }
      const pending = [from];
      const seen = new Set<string>();
      while (pending.length > 0) {
        const current = pending.pop() as string;
        if (seen.has(current)) continue;
        seen.add(current);
        if (isPrototypeModule(current)) {
          const message = check(current);
          if (message) this.error(message);
        }
        pending.push(...(parents.get(current) ?? []));
      }
    },
  };
}
