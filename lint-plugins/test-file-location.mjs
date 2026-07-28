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

// --- Plugin Export ---

const plugin = {
  meta: {
    name: "compozy-testing",
  },
  rules: {
    "tests-in-tests-dir": testsInTestsDir,
  },
};

export default plugin;
