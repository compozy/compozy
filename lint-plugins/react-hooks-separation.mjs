/**
 * OXC/ESLint Plugin: React Hooks Separation
 *
 * Enforces architectural separation between hooks and components:
 *
 * Rule 1: no-mixed-hooks-and-components
 *   - A file must not define both React components and custom hooks.
 *   - Reports on each hook definition found in a file that also contains components.
 *
 * Rule 2: hooks-in-hooks-folder
 *   - Custom hook definitions must live in a hooks/ directory or in a
 *     file named use-*.{ts,tsx}.
 *   - Reports on hook definitions found in files that don't match either pattern.
 */

// --- Helpers ---

function isPascalCase(name) {
  return typeof name === "string" && /^[A-Z][a-zA-Z0-9]*$/.test(name);
}

function isHookName(name) {
  return typeof name === "string" && /^use[A-Z]/.test(name);
}

const WRAPPER_NAMES = new Set(["forwardRef", "memo", "React.forwardRef", "React.memo"]);

function getCalleeName(callExpr) {
  if (!callExpr || callExpr.type !== "CallExpression") return null;
  const callee = callExpr.callee;
  if (callee.type === "Identifier") return callee.name;
  if (
    callee.type === "MemberExpression" &&
    callee.object.type === "Identifier" &&
    callee.property.type === "Identifier"
  ) {
    return `${callee.object.name}.${callee.property.name}`;
  }
  return null;
}

/**
 * Check if the init expression of a VariableDeclarator is function-like
 * (direct function or wrapped in memo/forwardRef).
 */
function isFunctionLikeInit(init) {
  if (!init) return false;
  if (init.type === "ArrowFunctionExpression" || init.type === "FunctionExpression") {
    return true;
  }
  if (init.type === "CallExpression") {
    const name = getCalleeName(init);
    if (name && WRAPPER_NAMES.has(name)) {
      return init.arguments?.length > 0 && isFunctionLikeInit(init.arguments[0]);
    }
  }
  return false;
}

/**
 * Check if a FunctionDeclaration is at module (top) level,
 * accounting for export wrappers.
 */
function isTopLevelFunction(node) {
  const parent = node.parent;
  return (
    parent.type === "Program" ||
    parent.type === "ExportNamedDeclaration" ||
    parent.type === "ExportDefaultDeclaration"
  );
}

/**
 * Check if a VariableDeclarator is at module (top) level,
 * accounting for export wrappers.
 */
function isTopLevelVariable(node) {
  const varDecl = node.parent;
  if (!varDecl || varDecl.type !== "VariableDeclaration") return false;
  const container = varDecl.parent;
  return (
    container.type === "Program" ||
    container.type === "ExportNamedDeclaration" ||
    container.type === "ExportDefaultDeclaration"
  );
}

/**
 * Convert a hook name like "useMyCustomHook" to kebab-case filename
 * "use-my-custom-hook.ts"
 */
function hookNameToKebabFile(name) {
  return name.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase() + ".ts";
}

// --- Shared collector logic ---

/**
 * Creates AST visitors that collect top-level hook and component definitions.
 * Returns { visitors, hooks, components } where hooks/components are arrays
 * populated during traversal.
 */
function createCollectors() {
  const hooks = [];
  const components = [];

  const visitors = {
    FunctionDeclaration(node) {
      if (!isTopLevelFunction(node)) return;
      const name = node.id?.name;
      if (!name) return;
      if (isHookName(name)) {
        hooks.push({ name, node });
      } else if (isPascalCase(name)) {
        components.push({ name, node });
      }
    },
    VariableDeclarator(node) {
      if (!isTopLevelVariable(node)) return;
      const name = node.id?.name;
      if (!name) return;

      // For hooks: any top-level variable with use* name is a hook definition
      // (covers arrow functions, factory calls, re-exports)
      if (isHookName(name)) {
        hooks.push({ name, node });
        return;
      }

      // For components: require function-like init (arrow fn, function expr,
      // or memo/forwardRef wrapper) to avoid false positives on constants
      if (isPascalCase(name) && isFunctionLikeInit(node.init)) {
        components.push({ name, node });
      }
    },
  };

  return { visitors, hooks, components };
}

// --- Rule 1: no-mixed-hooks-and-components ---

const noMixedHooksAndComponents = {
  meta: {
    type: "problem",
    docs: {
      description: "Disallow defining hooks and components in the same file",
    },
    messages: {
      mixed:
        "Hook '{{hookName}}' should not be defined in the same file as component '{{componentName}}'. Move it to a hooks/ folder (e.g., hooks/{{suggestedFile}}).",
    },
    schema: [],
  },
  create(context) {
    const { visitors, hooks, components } = createCollectors();

    return {
      ...visitors,
      "Program:exit"() {
        if (hooks.length === 0 || components.length === 0) return;
        for (const hook of hooks) {
          context.report({
            node: hook.node,
            messageId: "mixed",
            data: {
              hookName: hook.name,
              componentName: components[0].name,
              suggestedFile: hookNameToKebabFile(hook.name),
            },
          });
        }
      },
    };
  },
};

// --- Rule 2: hooks-in-hooks-folder ---

const hooksInHooksFolder = {
  meta: {
    type: "suggestion",
    docs: {
      description:
        "Enforce that custom hooks are defined in a hooks/ folder or use-*.{ts,tsx} file",
    },
    messages: {
      wrongLocation:
        "Hook '{{hookName}}' should be defined in a hooks/ folder (e.g., hooks/{{suggestedFile}}).",
    },
    schema: [],
  },
  create(context) {
    const filename = context.filename || context.getFilename?.() || "";

    // File is already in a valid hook location — nothing to check
    const isInHooksFolder = /[/\\]hooks[/\\]/.test(filename);
    const isHookFile = /[/\\]use-[a-z][a-z0-9-]*\.[tj]sx?$/i.test(filename);
    if (isInHooksFolder || isHookFile) return {};

    const hooks = [];

    return {
      FunctionDeclaration(node) {
        if (!isTopLevelFunction(node)) return;
        const name = node.id?.name;
        if (name && isHookName(name)) hooks.push({ name, node });
      },
      VariableDeclarator(node) {
        if (!isTopLevelVariable(node)) return;
        const name = node.id?.name;
        if (!name || !isHookName(name)) return;
        hooks.push({ name, node });
      },
      "Program:exit"() {
        for (const hook of hooks) {
          context.report({
            node: hook.node,
            messageId: "wrongLocation",
            data: {
              hookName: hook.name,
              suggestedFile: hookNameToKebabFile(hook.name),
            },
          });
        }
      },
    };
  },
};

// --- Plugin Export ---

const plugin = {
  meta: {
    name: "compozy-react-hooks",
  },
  rules: {
    "no-mixed-hooks-and-components": noMixedHooksAndComponents,
    "hooks-in-hooks-folder": hooksInHooksFolder,
  },
};

export default plugin;
