import { mkdir, readFile, readdir, unlink, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";

import ts from "typescript";

const repoRoot = process.cwd();
const writeMode = process.argv.includes("--write");

const workspaceRoots = [
  "web",
  "packages/ui",
  "packages/site",
  "sdk/typescript",
  "sdk/create-extension",
];

const ignoredDirNames = new Set([
  ".git",
  ".next",
  ".turbo",
  "coverage",
  "dist",
  "node_modules",
  "out",
]);

const testFilePattern = /\.(test|spec)\.(ts|tsx|js|jsx)$/;
const mockMethodNames = new Set(["doMock", "mock", "unmock"]);

function normalizeSeparators(value) {
  return value.split(path.sep).join("/");
}

function scriptKindForFile(filePath) {
  if (filePath.endsWith(".tsx")) {
    return ts.ScriptKind.TSX;
  }
  if (filePath.endsWith(".ts")) {
    return ts.ScriptKind.TS;
  }
  if (filePath.endsWith(".jsx")) {
    return ts.ScriptKind.JSX;
  }
  return ts.ScriptKind.JS;
}

function isRelativeSpecifier(value) {
  return value.startsWith("./") || value.startsWith("../");
}

function toRelativeSpecifier(fromFile, targetPath) {
  let relative = normalizeSeparators(path.relative(path.dirname(fromFile), targetPath));
  if (!relative.startsWith(".")) {
    relative = `./${relative}`;
  }
  return relative;
}

function resolveRelativeTarget(fromFile, specifier) {
  return path.resolve(path.dirname(fromFile), specifier);
}

function isImportMetaUrlExpression(node, sourceFile) {
  return node?.getText(sourceFile) === "import.meta.url";
}

function isMockMethodCall(node) {
  return (
    ts.isPropertyAccessExpression(node.expression) &&
    ts.isIdentifier(node.expression.expression) &&
    node.expression.expression.text === "vi" &&
    mockMethodNames.has(node.expression.name.text)
  );
}

function buildDestination(oldPath) {
  return path.join(path.dirname(oldPath), "__tests__", path.basename(oldPath));
}

function applyEdits(sourceText, edits) {
  let output = sourceText;
  for (const edit of edits.sort((left, right) => right.start - left.start)) {
    output = `${output.slice(0, edit.start)}${edit.value}${output.slice(edit.end)}`;
  }
  return output;
}

function collectSpecifierEdits(sourceFile, oldPath, newPath) {
  const edits = [];

  function queueStringLiteralUpdate(node) {
    const currentValue = node.text;
    if (!isRelativeSpecifier(currentValue)) {
      return;
    }

    const resolvedTarget = resolveRelativeTarget(oldPath, currentValue);
    const rewritten = toRelativeSpecifier(newPath, resolvedTarget);
    if (rewritten === currentValue) {
      return;
    }

    edits.push({
      start: node.getStart(sourceFile) + 1,
      end: node.getEnd() - 1,
      value: rewritten,
    });
  }

  function visit(node) {
    if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
      queueStringLiteralUpdate(node.moduleSpecifier);
    }

    if (
      ts.isExportDeclaration(node) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier)
    ) {
      queueStringLiteralUpdate(node.moduleSpecifier);
    }

    if (ts.isCallExpression(node)) {
      const [firstArg] = node.arguments;
      const isImportCall = node.expression.kind === ts.SyntaxKind.ImportKeyword;
      const isRequireCall = ts.isIdentifier(node.expression) && node.expression.text === "require";
      if (
        (isImportCall || isRequireCall || isMockMethodCall(node)) &&
        firstArg &&
        ts.isStringLiteralLike(firstArg)
      ) {
        queueStringLiteralUpdate(firstArg);
      }
    }

    if (
      ts.isNewExpression(node) &&
      ts.isIdentifier(node.expression) &&
      node.expression.text === "URL"
    ) {
      const [firstArg, secondArg] = node.arguments ?? [];
      if (
        firstArg &&
        secondArg &&
        ts.isStringLiteralLike(firstArg) &&
        isImportMetaUrlExpression(secondArg, sourceFile)
      ) {
        queueStringLiteralUpdate(firstArg);
      }
    }

    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return edits;
}

async function listMisplacedTests(dir) {
  const results = [];
  const entries = await readdir(dir, { withFileTypes: true });

  for (const entry of entries) {
    const entryPath = path.join(dir, entry.name);

    if (entry.isDirectory()) {
      if (ignoredDirNames.has(entry.name) || entry.name === "__tests__") {
        continue;
      }

      results.push(...(await listMisplacedTests(entryPath)));
      continue;
    }

    if (entry.isFile() && testFilePattern.test(entry.name)) {
      results.push(entryPath);
    }
  }

  return results;
}

async function rewriteAndMove(oldPath) {
  const newPath = buildDestination(oldPath);
  const sourceText = await readFile(oldPath, "utf8");
  const sourceFile = ts.createSourceFile(
    oldPath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    scriptKindForFile(oldPath)
  );
  const rewrittenSource = applyEdits(
    sourceText,
    collectSpecifierEdits(sourceFile, oldPath, newPath)
  );

  await mkdir(path.dirname(newPath), { recursive: true });
  await writeFile(newPath, rewrittenSource);
  await unlink(oldPath);
}

async function main() {
  const targets = [];
  for (const workspaceRoot of workspaceRoots) {
    targets.push(...(await listMisplacedTests(path.join(repoRoot, workspaceRoot))));
  }

  targets.sort();
  if (targets.length === 0) {
    console.log("No misplaced tests found.");
    return;
  }

  const destinationCollisions = targets
    .map(oldPath => ({ oldPath, newPath: buildDestination(oldPath) }))
    .filter(({ oldPath, newPath }) => oldPath !== newPath);

  for (const { oldPath, newPath } of destinationCollisions) {
    if (oldPath === newPath) {
      continue;
    }

    if (!writeMode) {
      console.log(
        `${normalizeSeparators(path.relative(repoRoot, oldPath))} -> ${normalizeSeparators(path.relative(repoRoot, newPath))}`
      );
      continue;
    }
  }

  if (!writeMode) {
    console.log(`Planned ${targets.length} test moves. Re-run with --write to apply.`);
    return;
  }

  for (const oldPath of targets) {
    await rewriteAndMove(oldPath);
  }

  console.log(`Migrated ${targets.length} tests into __tests__/ directories.`);
}

await main();
