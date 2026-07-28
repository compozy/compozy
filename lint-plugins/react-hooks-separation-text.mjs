import path from "node:path";
/**
 * React Hooks Separation Codemod
 *
 * Finds files with custom hooks that violate separation/location rules, then:
 * - Extracts each eligible hook to ../hooks/<hook-file>.ts
 * - Removes extracted hook declarations from the source file
 * - Adds imports (and re-exports when needed) back to the source file
 * - Organizes imports (remove unused) with TypeScript language service
 * - Runs oxfmt on touched files (write mode)
 *
 * Usage:
 *   node lint-plugins/react-hooks-separation-codemod.mjs
 *   node lint-plugins/react-hooks-separation-codemod.mjs --write
 *   node lint-plugins/react-hooks-separation-codemod.mjs --write --path packages/electron/src/renderer/src/components
 *   node lint-plugins/react-hooks-separation-codemod.mjs --write --overwrite
 */
import ts from "typescript";
import fs from "node:fs";
import process from "node:process";

import {
  collectReferencedIdentifiers,
  createSourceFile,
  hasExportModifier,
} from "./react-hooks-separation-ast.mjs";

export function toImportPath(fromFilePath, toFilePath) {
  let relativePath = path
    .relative(path.dirname(fromFilePath), toFilePath)
    .replace(/\.[^.]+$/, "")
    .replace(/\\/g, "/");

  if (!relativePath.startsWith(".")) {
    relativePath = `./${relativePath}`;
  }

  return relativePath;
}

function applyTextChanges(sourceText, textChanges) {
  if (textChanges.length === 0) return sourceText;

  const orderedChanges = [...textChanges].sort((a, b) => b.span.start - a.span.start);
  let nextText = sourceText;
  for (const change of orderedChanges) {
    const start = change.span.start;
    const end = change.span.start + change.span.length;
    nextText = `${nextText.slice(0, start)}${change.newText}${nextText.slice(end)}`;
  }

  return nextText;
}

export function organizeImports(filePath, sourceText) {
  const absoluteFilePath = path.resolve(filePath);
  const fileMap = new Map([[absoluteFilePath, sourceText]]);

  const languageServiceHost = {
    getCompilationSettings: () => ({
      allowJs: true,
      jsx: ts.JsxEmit.ReactJSX,
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ESNext,
    }),
    getScriptFileNames: () => [absoluteFilePath],
    getScriptVersion: () => "1",
    getScriptSnapshot: name => {
      if (fileMap.has(name)) {
        return ts.ScriptSnapshot.fromString(fileMap.get(name));
      }
      if (!fs.existsSync(name)) return undefined;
      return ts.ScriptSnapshot.fromString(fs.readFileSync(name, "utf8"));
    },
    getCurrentDirectory: () => process.cwd(),
    getDefaultLibFileName: options => ts.getDefaultLibFilePath(options),
    fileExists: fs.existsSync,
    readFile: name => {
      if (fileMap.has(name)) return fileMap.get(name);
      if (!fs.existsSync(name)) return undefined;
      return fs.readFileSync(name, "utf8");
    },
    readDirectory: (...args) => ts.sys.readDirectory(...args),
    directoryExists: directoryPath => {
      try {
        return fs.statSync(directoryPath).isDirectory();
      } catch {
        return false;
      }
    },
    getDirectories: directoryPath => ts.sys.getDirectories(directoryPath),
  };

  const languageService = ts.createLanguageService(languageServiceHost);
  const organizeChanges = languageService.organizeImports(
    { type: "file", fileName: absoluteFilePath },
    {},
    {}
  );
  languageService.dispose();

  const ownFileChanges = organizeChanges.find(
    change => path.resolve(change.fileName) === absoluteFilePath
  );
  if (!ownFileChanges || ownFileChanges.textChanges.length === 0) return sourceText;

  return applyTextChanges(sourceText, ownFileChanges.textChanges);
}

export function ensureTrailingNewline(text) {
  return text.endsWith("\n") ? text : `${text}\n`;
}

export function removeRanges(sourceText, ranges) {
  if (ranges.length === 0) return sourceText;

  const orderedRanges = [...ranges].sort((a, b) => b.start - a.start);
  let nextText = sourceText;

  for (const range of orderedRanges) {
    nextText = `${nextText.slice(0, range.start)}${nextText.slice(range.end)}`;
  }

  return nextText;
}

export function ensureExportedHookStatement(statementNode, sourceFile) {
  const statementText = statementNode.getText(sourceFile).trim();
  if (hasExportModifier(statementNode)) return statementText;
  return `export ${statementText}`;
}

export function buildHookFileContent({
  hookFilePath,
  importStatements,
  hookStatementText,
  hookDependencyImports,
  localValueImports,
}) {
  const contentParts = [];
  if (importStatements.length > 0) {
    contentParts.push(importStatements.join("\n"));
  }
  if (hookDependencyImports.length > 0) {
    contentParts.push(hookDependencyImports.join("\n"));
  }
  if (localValueImports.length > 0) {
    contentParts.push(localValueImports.join("\n"));
  }
  contentParts.push(hookStatementText);

  let hookSource = contentParts.join("\n\n");
  hookSource = organizeImports(hookFilePath, hookSource);
  return ensureTrailingNewline(hookSource);
}

function findImportsInsertionPosition(sourceFile) {
  let insertionPoint = 0;
  for (const statement of sourceFile.statements) {
    if (ts.isExpressionStatement(statement) && ts.isStringLiteral(statement.expression)) {
      insertionPoint = statement.getEnd();
      continue;
    }
    if (ts.isImportDeclaration(statement)) {
      insertionPoint = statement.getEnd();
      continue;
    }
    break;
  }
  return insertionPoint;
}

export function injectHookImportsAndExports(sourceText, componentFilePath, extractedHooks) {
  if (extractedHooks.length === 0) return sourceText;

  const updatedSourceFile = createSourceFile(componentFilePath, sourceText);
  const referencedNames = collectReferencedIdentifiers(updatedSourceFile);

  const declarationsToInsert = [];
  const seenLines = new Set();

  for (const hook of extractedHooks) {
    const hookImportPath = toImportPath(componentFilePath, hook.hookFilePath);
    if (referencedNames.has(hook.name)) {
      const importLine = `import { ${hook.name} } from "${hookImportPath}";`;
      if (!seenLines.has(importLine)) {
        declarationsToInsert.push(importLine);
        seenLines.add(importLine);
      }
    }

    if (hook.isExported) {
      const exportLine = `export { ${hook.name} } from "${hookImportPath}";`;
      if (!seenLines.has(exportLine)) {
        declarationsToInsert.push(exportLine);
        seenLines.add(exportLine);
      }
    }
  }

  if (declarationsToInsert.length === 0) return sourceText;

  const insertionPoint = findImportsInsertionPosition(updatedSourceFile);
  const needsLeadingNewline = insertionPoint > 0 && sourceText[insertionPoint - 1] !== "\n";
  const needsTrailingNewline =
    insertionPoint < sourceText.length && sourceText[insertionPoint] !== "\n";

  const block = `${needsLeadingNewline ? "\n" : ""}${declarationsToInsert.join("\n")}${
    needsTrailingNewline ? "\n" : "\n\n"
  }`;

  return `${sourceText.slice(0, insertionPoint)}${block}${sourceText.slice(insertionPoint)}`;
}

function ensureExportModifier(statementNode, sourceFile, sourceText) {
  if (hasExportModifier(statementNode)) {
    return sourceText;
  }

  const insertPosition = statementNode.getStart(sourceFile);
  return `${sourceText.slice(0, insertPosition)}export ${sourceText.slice(insertPosition)}`;
}

export function ensureExportModifiers(sourceText, sourceFile, statements) {
  if (statements.length === 0) {
    return sourceText;
  }

  const orderedStatements = [...statements].sort(
    (left, right) => right.getStart() - left.getStart()
  );
  let nextSource = sourceText;

  for (const statement of orderedStatements) {
    nextSource = ensureExportModifier(statement, sourceFile, nextSource);
  }

  return nextSource;
}
