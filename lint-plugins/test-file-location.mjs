/**
 * OXC/ESLint Plugin: Test File Location
 *
 * Enforces that test files (.test.ts, .test.tsx, .spec.ts, .spec.tsx) are placed inside
 * __tests__/ directories rather than co-located with source files.
 *
 * Rule: tests-in-tests-dir
 *   - Checks context.filename on the Program node (once per file).
 *   - If the file matches *.(test|spec).{ts,tsx,js,jsx} and is NOT inside a
 *     __tests__/ directory, reports an error.
 *   - Allows configuration of the required directory name (default: __tests__).
 */

// --- Helpers ---

const TEST_FILE_RE = /\.(test|spec)\.(ts|tsx|js|jsx)$/;

function isTestFile(filename) {
  return TEST_FILE_RE.test(filename);
}

function isInsideTestsDir(filename, dirName) {
  // Check if any segment of the path is the required directory name.
  // Use both / and \ to handle cross-platform paths.
  const segments = filename.split(/[/\\]/);
  return segments.includes(dirName);
}

// --- Rule Definition ---

const testsInTestsDir = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Enforce test and spec files to be placed inside __tests__/ directories",
      recommended: false,
    },
    messages: {
      misplacedTestFile:
        "Test or spec file '{{fileName}}' must be inside a '{{dirName}}/' directory. " +
        "Move it to '{{suggestedPath}}'.",
    },
    schema: [
      {
        type: "object",
        properties: {
          dirName: {
            type: "string",
            description: "The required directory name for test files (default: __tests__)",
          },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    const options = context.options[0] || {};
    const dirName = options.dirName ?? "__tests__";

    return {
      Program(node) {
        const filename = context.filename;
        if (!filename || !isTestFile(filename)) {
          return;
        }

        if (isInsideTestsDir(filename, dirName)) {
          return;
        }

        // Extract just the file name for the message
        const segments = filename.split(/[/\\]/);
        const fileName = segments[segments.length - 1];

        // Build suggested path: insert __tests__/ before the file name
        const parentSegments = segments.slice(0, -1);
        const suggestedPath = [...parentSegments, dirName, fileName].join("/");

        context.report({
          node,
          messageId: "misplacedTestFile",
          data: {
            fileName,
            dirName,
            suggestedPath,
          },
        });
      },
    };
  },
};

/** Accept only finite positive literal deadlines within the owning suite budget. */
function positiveDeadlineWithin(node, budget) {
  const value =
    node?.type === "UnaryExpression" && node.operator === "-" ? -node.argument?.value : node?.value;
  return typeof value === "number" && Number.isFinite(value) && value > 0 && value <= budget;
}

/** Resolve literals and immutable local aliases without evaluating arbitrary expressions. */
function staticValue(node, sourceCode, seen = new Set()) {
  if (!node || seen.has(node)) return undefined;
  seen.add(node);
  if (node.type === "TemplateLiteral" && node.expressions.length === 0) {
    return node.quasis[0]?.value.cooked ?? undefined;
  }
  if (node.type !== "Identifier") return node.value;
  for (let scope = sourceCode?.getScope(node); scope; scope = scope.upper) {
    const variable = scope.set.get(node.name);
    if (!variable) continue;
    const definition = variable.defs[0];
    if (
      variable.defs.length !== 1 ||
      definition.parent?.kind !== "const" ||
      definition.node.id.type !== "Identifier"
    )
      return undefined;
    return staticValue(definition.node.init, sourceCode, seen);
  }
  return undefined;
}

/** Resolve property keys while leaving dynamic computed expressions unknown. */
function propertyName(node, property = node.property, sourceCode) {
  return node.computed ? staticValue(property, sourceCode) : (property?.name ?? property?.value);
}

/** Follow supported Vitest modifiers and factories back to a test or suite root. */
function isTestFunction(node, roots = ["test", "it"], sourceCode) {
  if (node?.type === "Identifier") return roots.includes(node.name);
  if (node?.type === "MemberExpression") {
    return (
      ["concurrent", "sequential", "only", "skip", "todo", "fails"].includes(
        propertyName(node, node.property, sourceCode)
      ) && isTestFunction(node.object, roots, sourceCode)
    );
  }
  const factory =
    node?.type === "CallExpression"
      ? node.callee
      : node?.type === "TaggedTemplateExpression"
        ? node.tag
        : undefined;
  return (
    factory?.type === "MemberExpression" &&
    ["each", "for", "skipIf", "runIf"].includes(
      propertyName(factory, factory.property, sourceCode)
    ) &&
    isTestFunction(factory.object, roots, sourceCode)
  );
}

/** Find unconditional skips anywhere in an already recognized declaration chain. */
function hasSkippedDeclaration(node, sourceCode) {
  if (node?.type === "MemberExpression") {
    return (
      propertyName(node, node.property, sourceCode) === "skip" ||
      hasSkippedDeclaration(node.object, sourceCode)
    );
  }
  if (node?.type === "CallExpression") {
    const factory = node.callee;
    if (factory.type === "MemberExpression") {
      const name = propertyName(factory, factory.property, sourceCode);
      const condition = ["skipIf", "runIf"].includes(name)
        ? staticValue(node.arguments?.[0], sourceCode)
        : undefined;
      if ((name === "skipIf" && condition === true) || (name === "runIf" && condition === false))
        return true;
    }
    return hasSkippedDeclaration(factory, sourceCode);
  }
  if (node?.type === "TaggedTemplateExpression") return hasSkippedDeclaration(node.tag, sourceCode);
  return false;
}

/** Inspect inline option spreads and reject spreads whose overrides cannot be checked. */
function unsafeOptions(options, budget, sourceCode) {
  return options.properties.some(property => {
    if (property.type === "SpreadElement") {
      return (
        property.argument.type !== "ObjectExpression" ||
        unsafeOptions(property.argument, budget, sourceCode)
      );
    }
    const key = propertyName(property, property.key, sourceCode);
    if (property.computed && key === undefined) return true;
    return (
      key === "retry" || (key === "timeout" && !positiveDeadlineWithin(property.value, budget))
    );
  });
}

// Unit tests should synchronize with observable completion. Real clock/process
// integration contracts can document a local exception with an eslint directive.
const deterministicTests = {
  meta: {
    type: "problem",
    schema: [
      {
        type: "object",
        properties: { defaultTimeout: { type: "number", minimum: 1 } },
        additionalProperties: false,
      },
    ],
    messages: {
      nondeterministic:
        "Use observable completion instead of sleeps, retries, unjustified timeout increases or unconditional skips. Document a real timing contract in a local lint exception.",
    },
  },
  create(context) {
    const filename = context.filename.replaceAll("\\", "/");
    if (!/\.(test|spec)\.[cm]?[jt]sx?$/.test(filename)) return {};
    if (/\/(web|desktop)\/e2e\//.test(filename) && /\.spec\./.test(filename)) return {};
    const defaultTimeout = context.options?.[0]?.defaultTimeout ?? 5000;
    return {
      CallExpression(node) {
        const callee = node.callee;
        const name = callee.type === "Identifier" ? callee.name : undefined;
        const member =
          callee.type === "MemberExpression" && !callee.computed ? callee.property.name : undefined;
        const testCall = isTestFunction(callee, undefined, context.sourceCode);
        const options = testCall
          ? node.arguments.find(arg => arg.type === "ObjectExpression")
          : undefined;
        const override = options && unsafeOptions(options, defaultTimeout, context.sourceCode);
        const numericSleep =
          ["sleep", "delay"].includes(name) && typeof node.arguments[0]?.value === "number";
        const timeout =
          testCall &&
          !options &&
          node.arguments.length >= 3 &&
          !positiveDeadlineWithin(node.arguments[2], defaultTimeout);
        const skipped =
          isTestFunction(callee, ["test", "it", "describe"], context.sourceCode) &&
          hasSkippedDeclaration(callee, context.sourceCode);
        if (numericSleep || member === "waitForTimeout" || override || timeout || skipped) {
          context.report({ node, messageId: "nondeterministic" });
        }
      },
    };
  },
};

// --- Plugin Export ---

const plugin = {
  meta: {
    name: "compozy-testing",
  },
  rules: {
    "tests-in-tests-dir": testsInTestsDir,
    "deterministic-tests": deterministicTests,
  },
};

export default plugin;
