#!/usr/bin/env node

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

import fs from "node:fs";
import path from "node:path";
import process from "node:process";
import { spawnSync } from "node:child_process";

import {
  buildHookFileContent,
  ensureExportModifiers,
  ensureExportedHookStatement,
  ensureTrailingNewline,
  injectHookImportsAndExports,
  organizeImports,
  removeRanges,
  toImportPath,
} from "./react-hooks-separation-text.mjs";
import {
  collectEligibleHooks,
  hookNameToKebabFile,
  isValidHookLocation,
} from "./react-hooks-separation-dependencies.mjs";
import { collectTopLevelMetadata, createSourceFile } from "./react-hooks-separation-ast.mjs";

const DEFAULT_TARGET = "packages/electron/src/renderer/src";

const EXCLUDED_DIRS = new Set([".git", ".turbo", "build", "coverage", "dist", "node_modules"]);

function transformFile(filePath, options) {
  const originalSource = fs.readFileSync(filePath, "utf8");
  const sourceFile = createSourceFile(filePath, originalSource);
  const metadata = collectTopLevelMetadata(sourceFile);
  const hasHooks = metadata.hooks.length > 0;
  const hasComponents = metadata.components.length > 0;
  const validHookLocation = isValidHookLocation(filePath);

  if (!hasHooks || (!hasComponents && validHookLocation)) {
    return {
      changed: false,
      filePath,
      warnings: [],
      hookOutputs: [],
      componentSource: originalSource,
    };
  }

  const warnings = [];
  const eligibleHooks = collectEligibleHooks({
    hooks: metadata.hooks,
    importedNames: metadata.importedNames,
    moduleDeclaredNames: metadata.moduleDeclaredNames,
    moduleDeclarationMap: metadata.moduleDeclarationMap,
    filePath,
    warnings,
  });
  if (eligibleHooks.length === 0) {
    return {
      changed: false,
      filePath,
      warnings,
      hookOutputs: [],
      componentSource: originalSource,
    };
  }

  const plannedHooks = eligibleHooks
    .map(analysis => {
      const hookFilePath = path.resolve(
        path.dirname(filePath),
        "..",
        "hooks",
        hookNameToKebabFile(analysis.hook.name)
      );
      if (path.resolve(hookFilePath) === path.resolve(filePath)) {
        warnings.push(
          `[skip] ${filePath}: hook "${analysis.hook.name}" resolves to its own source file target (${path.relative(
            process.cwd(),
            hookFilePath
          )})`
        );
        return null;
      }
      const dependencyImports = analysis.hookDependencies.map(dependencyName => {
        const dependencyFilePath = path.resolve(
          path.dirname(filePath),
          "..",
          "hooks",
          hookNameToKebabFile(dependencyName)
        );
        return `import { ${dependencyName} } from "${toImportPath(hookFilePath, dependencyFilePath)}";`;
      });
      const localValueDependencies = [...analysis.valueDependencies].sort((left, right) =>
        left.localeCompare(right)
      );
      const localValueImports =
        localValueDependencies.length === 0
          ? []
          : [
              `import { ${localValueDependencies.join(", ")} } from "${toImportPath(
                hookFilePath,
                filePath
              )}";`,
            ];

      const typeDependencyStatements = analysis.typeDependencies
        .map(dependencyName => metadata.moduleDeclarationMap.get(dependencyName))
        .filter(Boolean)
        .sort((left, right) => left.statement.getStart() - right.statement.getStart())
        .map(dependencyEntry => dependencyEntry.statement.getText(sourceFile));

      const hookSource = buildHookFileContent({
        hookFilePath,
        importStatements: metadata.importStatements,
        hookStatementText: [
          ...typeDependencyStatements,
          ensureExportedHookStatement(analysis.hook.statement, sourceFile),
        ].join("\n\n"),
        hookDependencyImports: dependencyImports,
        localValueImports,
      });

      return {
        ...analysis.hook,
        localValueDependencies,
        hookFilePath,
        hookSource,
      };
    })
    .filter(Boolean);

  if (plannedHooks.length === 0) {
    return {
      changed: false,
      filePath,
      warnings,
      hookOutputs: [],
      componentSource: originalSource,
    };
  }

  const hooksToExtract = [];
  for (const plannedHook of plannedHooks) {
    if (!fs.existsSync(plannedHook.hookFilePath)) {
      hooksToExtract.push(plannedHook);
      continue;
    }

    const existingHookSource = fs.readFileSync(plannedHook.hookFilePath, "utf8");
    if (existingHookSource === plannedHook.hookSource) {
      hooksToExtract.push(plannedHook);
      continue;
    }

    if (options.overwrite) {
      hooksToExtract.push(plannedHook);
      continue;
    }

    warnings.push(
      `[skip] ${filePath}: target hook file already exists with different content (${path.relative(
        process.cwd(),
        plannedHook.hookFilePath
      )}). Re-run with --overwrite to replace it.`
    );
  }

  if (hooksToExtract.length === 0) {
    return {
      changed: false,
      filePath,
      warnings,
      hookOutputs: [],
      componentSource: originalSource,
    };
  }

  const removalRanges = hooksToExtract.map(hook => ({
    start: hook.statement.getFullStart(),
    end: hook.statement.getEnd(),
  }));
  const dependencyNamesToExport = new Set();
  for (const hook of hooksToExtract) {
    for (const dependencyName of hook.localValueDependencies) {
      dependencyNamesToExport.add(dependencyName);
    }
  }

  let componentSource = removeRanges(originalSource, removalRanges);

  if (dependencyNamesToExport.size > 0) {
    const nextSourceFile = createSourceFile(filePath, componentSource);
    const nextMetadata = collectTopLevelMetadata(nextSourceFile);
    const statementsToExport = [];
    const seenStatements = new Set();
    for (const dependencyName of dependencyNamesToExport) {
      const declarationEntry = nextMetadata.moduleDeclarationMap.get(dependencyName);
      if (!declarationEntry || declarationEntry.declarationKind !== "value") {
        continue;
      }
      const statementKey = `${declarationEntry.statement.pos}:${declarationEntry.statement.end}`;
      if (seenStatements.has(statementKey)) {
        continue;
      }
      seenStatements.add(statementKey);
      statementsToExport.push(declarationEntry.statement);
    }
    componentSource = ensureExportModifiers(componentSource, nextSourceFile, statementsToExport);
  }

  componentSource = injectHookImportsAndExports(componentSource, filePath, hooksToExtract);
  componentSource = organizeImports(filePath, componentSource);
  componentSource = ensureTrailingNewline(componentSource);

  const changed = componentSource !== originalSource || hooksToExtract.length > 0;
  return {
    changed,
    filePath,
    warnings,
    hookOutputs: hooksToExtract,
    componentSource,
  };
}

function isTargetSourceFile(filePath) {
  if (filePath.endsWith(".d.ts")) return false;
  if (!filePath.endsWith(".ts") && !filePath.endsWith(".tsx")) return false;
  return true;
}

function walkTargetFiles(targetPath, collector) {
  if (!fs.existsSync(targetPath)) return;

  const stats = fs.statSync(targetPath);
  if (stats.isFile()) {
    if (isTargetSourceFile(targetPath)) {
      collector.push(path.resolve(targetPath));
    }
    return;
  }

  if (!stats.isDirectory()) return;

  const entries = fs.readdirSync(targetPath, { withFileTypes: true });
  for (const entry of entries) {
    if (entry.isDirectory() && EXCLUDED_DIRS.has(entry.name)) continue;
    walkTargetFiles(path.join(targetPath, entry.name), collector);
  }
}

function parseArguments(argv) {
  const options = {
    write: false,
    overwrite: false,
    runOxfmt: true,
    targets: [],
    help: false,
  };

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "--write" || argument === "--apply") {
      options.write = true;
      continue;
    }
    if (argument === "--overwrite") {
      options.overwrite = true;
      continue;
    }
    if (argument === "--no-oxfmt") {
      options.runOxfmt = false;
      continue;
    }
    if (argument === "--help" || argument === "-h") {
      options.help = true;
      continue;
    }
    if (argument === "--path") {
      const nextValue = argv[index + 1];
      if (!nextValue || nextValue.startsWith("--")) {
        throw new Error("--path requires a value");
      }
      options.targets.push(nextValue);
      index += 1;
      continue;
    }

    options.targets.push(argument);
  }

  return options;
}

function printHelp() {
  console.log("React Hooks Separation Codemod");
  console.log("");
  console.log("Usage:");
  console.log("  node lint-plugins/react-hooks-separation-codemod.mjs [options] [paths...]");
  console.log("");
  console.log("Options:");
  console.log("  --write, --apply   Write changes to disk");
  console.log("  --overwrite        Overwrite existing hook files when generated content differs");
  console.log("  --no-oxfmt         Skip oxfmt (write mode only)");
  console.log("  --path <path>      Add target path (file or directory)");
  console.log("  -h, --help         Show this help");
  console.log("");
  console.log(`Default target: ${DEFAULT_TARGET}`);
}

function runOxfmt(filePaths) {
  if (filePaths.length === 0) return;

  const sortedFiles = [...new Set(filePaths)].sort((left, right) => left.localeCompare(right));
  const formatResult = spawnSync("pnpm", ["exec", "oxfmt", ...sortedFiles], {
    stdio: "inherit",
    cwd: process.cwd(),
  });

  if (formatResult.status !== 0) {
    throw new Error("oxfmt failed");
  }
}

function main() {
  const options = parseArguments(process.argv.slice(2));
  if (options.help) {
    printHelp();
    return;
  }

  const targetPaths = options.targets.length > 0 ? options.targets : [DEFAULT_TARGET];
  const files = [];
  for (const targetPath of targetPaths) {
    walkTargetFiles(path.resolve(targetPath), files);
  }

  const uniqueFiles = [...new Set(files)].sort((left, right) => left.localeCompare(right));
  if (uniqueFiles.length === 0) {
    console.log("No target files found.");
    return;
  }

  const touchedFiles = new Set();
  const warnings = [];
  let transformedFiles = 0;
  let createdHookFiles = 0;

  for (const filePath of uniqueFiles) {
    const result = transformFile(filePath, options);
    for (const warning of result.warnings) {
      warnings.push(warning);
    }
    if (!result.changed) continue;

    transformedFiles += 1;
    const relativeComponentPath = path.relative(process.cwd(), result.filePath);

    if (!options.write) {
      const hookTargets = result.hookOutputs.map(hookOutput =>
        path.relative(process.cwd(), hookOutput.hookFilePath)
      );
      console.log(`[plan] ${relativeComponentPath}`);
      for (const hookTarget of hookTargets) {
        console.log(`  -> ${hookTarget}`);
      }
      continue;
    }

    for (const hookOutput of result.hookOutputs) {
      fs.mkdirSync(path.dirname(hookOutput.hookFilePath), { recursive: true });
      const existed = fs.existsSync(hookOutput.hookFilePath);
      fs.writeFileSync(hookOutput.hookFilePath, hookOutput.hookSource, "utf8");
      if (!existed) {
        createdHookFiles += 1;
      }
      touchedFiles.add(path.resolve(hookOutput.hookFilePath));
    }

    fs.writeFileSync(result.filePath, result.componentSource, "utf8");
    touchedFiles.add(path.resolve(result.filePath));
  }

  for (const warning of warnings) {
    console.warn(warning);
  }

  if (options.write && options.runOxfmt && touchedFiles.size > 0) {
    runOxfmt([...touchedFiles]);
  }

  if (options.write) {
    console.log(
      `Done. Transformed ${transformedFiles} file(s), created ${createdHookFiles} new hook file(s).`
    );
  } else {
    console.log(
      `Dry run complete. ${transformedFiles} file(s) would be transformed. Re-run with --write to apply.`
    );
  }
}

try {
  main();
} catch (error) {
  if (error instanceof Error) {
    console.error(`Failed: ${error.message}`);
    if (error.stack) {
      console.error(error.stack);
    }
  } else {
    console.error(`Failed: ${String(error)}`);
  }
  process.exit(1);
}
