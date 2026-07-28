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

import { isHookName, isPascalCase } from "./react-hooks-separation-dependencies.mjs";

const WRAPPER_NAMES = new Set(["forwardRef", "memo", "React.forwardRef", "React.memo"]);

export function hasExportModifier(node) {
  return Boolean(node.modifiers?.some(modifier => modifier.kind === ts.SyntaxKind.ExportKeyword));
}

function getCalleeName(callExpression) {
  const callee = callExpression.expression;
  if (ts.isIdentifier(callee)) return callee.text;
  if (
    ts.isPropertyAccessExpression(callee) &&
    ts.isIdentifier(callee.expression) &&
    ts.isIdentifier(callee.name)
  ) {
    return `${callee.expression.text}.${callee.name.text}`;
  }
  return null;
}

function isFunctionLikeInit(initializer) {
  if (!initializer) return false;
  if (ts.isArrowFunction(initializer) || ts.isFunctionExpression(initializer)) return true;

  if (ts.isCallExpression(initializer)) {
    const calleeName = getCalleeName(initializer);
    if (calleeName && WRAPPER_NAMES.has(calleeName) && initializer.arguments.length > 0) {
      return isFunctionLikeInit(initializer.arguments[0]);
    }
  }

  return false;
}

function getScriptKind(filePath) {
  if (filePath.endsWith(".tsx")) return ts.ScriptKind.TSX;
  if (filePath.endsWith(".ts")) return ts.ScriptKind.TS;
  if (filePath.endsWith(".jsx")) return ts.ScriptKind.JSX;
  if (filePath.endsWith(".js")) return ts.ScriptKind.JS;
  return ts.ScriptKind.Unknown;
}

export function createSourceFile(filePath, sourceText) {
  return ts.createSourceFile(
    filePath,
    sourceText,
    ts.ScriptTarget.Latest,
    true,
    getScriptKind(filePath)
  );
}

function addBindingNames(nameNode, collector) {
  if (ts.isIdentifier(nameNode)) {
    collector.add(nameNode.text);
    return;
  }

  if (ts.isArrayBindingPattern(nameNode) || ts.isObjectBindingPattern(nameNode)) {
    for (const element of nameNode.elements) {
      if (ts.isOmittedExpression(element)) continue;
      addBindingNames(element.name, collector);
    }
  }
}

export function collectDeclaredNames(rootNode) {
  const declaredNames = new Set();

  function visit(node) {
    if (ts.isVariableDeclaration(node) || ts.isParameter(node) || ts.isBindingElement(node)) {
      addBindingNames(node.name, declaredNames);
    } else if (
      ts.isFunctionDeclaration(node) ||
      ts.isFunctionExpression(node) ||
      ts.isClassDeclaration(node) ||
      ts.isClassExpression(node) ||
      ts.isInterfaceDeclaration(node) ||
      ts.isTypeAliasDeclaration(node) ||
      ts.isEnumDeclaration(node) ||
      ts.isTypeParameterDeclaration(node) ||
      ts.isImportEqualsDeclaration(node)
    ) {
      if (node.name && ts.isIdentifier(node.name)) {
        declaredNames.add(node.name.text);
      }
    } else if (ts.isCatchClause(node) && node.variableDeclaration) {
      addBindingNames(node.variableDeclaration.name, declaredNames);
    }

    ts.forEachChild(node, visit);
  }

  visit(rootNode);
  return declaredNames;
}

function isReferenceIdentifier(identifierNode) {
  const parent = identifierNode.parent;
  if (!parent) return false;

  if (ts.isPropertyAccessExpression(parent) && parent.name === identifierNode) return false;
  if (ts.isQualifiedName(parent) && parent.right === identifierNode) return false;

  if (
    (ts.isPropertyAssignment(parent) ||
      ts.isPropertyDeclaration(parent) ||
      ts.isPropertySignature(parent) ||
      ts.isMethodDeclaration(parent) ||
      ts.isMethodSignature(parent) ||
      ts.isEnumMember(parent)) &&
    parent.name === identifierNode
  ) {
    return false;
  }

  if (
    (ts.isVariableDeclaration(parent) ||
      ts.isParameter(parent) ||
      ts.isBindingElement(parent) ||
      ts.isImportClause(parent) ||
      ts.isImportSpecifier(parent) ||
      ts.isNamespaceImport(parent) ||
      ts.isNamespaceExport(parent) ||
      ts.isExportSpecifier(parent)) &&
    parent.name === identifierNode
  ) {
    return false;
  }

  if (
    (ts.isFunctionDeclaration(parent) ||
      ts.isFunctionExpression(parent) ||
      ts.isClassDeclaration(parent) ||
      ts.isClassExpression(parent) ||
      ts.isInterfaceDeclaration(parent) ||
      ts.isTypeAliasDeclaration(parent) ||
      ts.isEnumDeclaration(parent) ||
      ts.isTypeParameterDeclaration(parent) ||
      ts.isImportEqualsDeclaration(parent)) &&
    parent.name === identifierNode
  ) {
    return false;
  }

  if (
    (ts.isLabeledStatement(parent) || ts.isBreakOrContinueStatement(parent)) &&
    parent.label === identifierNode
  ) {
    return false;
  }

  return true;
}

export function collectReferencedIdentifiers(rootNode, excludedRanges = []) {
  const references = new Set();

  function isExcluded(node) {
    return excludedRanges.some(
      range => node.getStart() >= range.start && node.getEnd() <= range.end
    );
  }

  function visit(node) {
    if (isExcluded(node)) return;

    if (ts.isIdentifier(node) && isReferenceIdentifier(node)) {
      references.add(node.text);
    }

    ts.forEachChild(node, visit);
  }

  visit(rootNode);
  return references;
}

export function collectTopLevelMetadata(sourceFile) {
  const hooks = [];
  const components = [];
  const importStatements = [];
  const importedNames = new Set();
  const moduleDeclarationMap = new Map();
  const moduleDeclaredNames = new Set();

  function registerDeclaration(name, statement, declarationKind) {
    moduleDeclaredNames.add(name);
    if (!moduleDeclarationMap.has(name)) {
      moduleDeclarationMap.set(name, { statement, declarationKind });
    }
  }

  for (const statement of sourceFile.statements) {
    if (ts.isImportDeclaration(statement)) {
      importStatements.push(statement.getText(sourceFile));

      const importClause = statement.importClause;
      if (importClause?.name) {
        importedNames.add(importClause.name.text);
      }

      if (importClause?.namedBindings) {
        if (ts.isNamespaceImport(importClause.namedBindings)) {
          importedNames.add(importClause.namedBindings.name.text);
        } else if (ts.isNamedImports(importClause.namedBindings)) {
          for (const specifier of importClause.namedBindings.elements) {
            importedNames.add(specifier.name.text);
          }
        }
      }
    }

    if (ts.isFunctionDeclaration(statement) && statement.name) {
      const functionName = statement.name.text;
      registerDeclaration(functionName, statement, "value");

      if (isHookName(functionName)) {
        hooks.push({
          kind: "function",
          name: functionName,
          statement,
          isExported: hasExportModifier(statement),
          canExtract: true,
        });
      } else if (isPascalCase(functionName)) {
        components.push({ name: functionName, node: statement });
      }
      continue;
    }

    if (ts.isVariableStatement(statement)) {
      const declarations = statement.declarationList.declarations;
      for (const declaration of declarations) {
        if (!ts.isIdentifier(declaration.name)) continue;

        const declarationName = declaration.name.text;
        registerDeclaration(declarationName, statement, "value");

        if (isHookName(declarationName)) {
          hooks.push({
            kind: "variable",
            name: declarationName,
            statement,
            declaration,
            isExported: hasExportModifier(statement),
            canExtract: declarations.length === 1,
          });
        } else if (isPascalCase(declarationName) && isFunctionLikeInit(declaration.initializer)) {
          components.push({ name: declarationName, node: declaration });
        }
      }
      continue;
    }

    if (ts.isClassDeclaration(statement) && statement.name) {
      registerDeclaration(statement.name.text, statement, "value");
      continue;
    }

    if (ts.isTypeAliasDeclaration(statement) || ts.isInterfaceDeclaration(statement)) {
      registerDeclaration(statement.name.text, statement, "type");
      continue;
    }

    if (ts.isEnumDeclaration(statement)) {
      registerDeclaration(statement.name.text, statement, "value");
    }
  }

  return {
    hooks,
    components,
    importStatements,
    importedNames,
    moduleDeclaredNames,
    moduleDeclarationMap,
  };
}
