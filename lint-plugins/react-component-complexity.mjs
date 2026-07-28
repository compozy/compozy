/**
 * OXC/ESLint Plugin: React Component Complexity
 *
 * Detects React components with too much behavior/logic mixed into the
 * component body and suggests extracting logic into custom hooks.
 *
 * This enforces the architectural pattern: components should be primarily
 * visual, delegating behavior to custom hooks.
 *
 * Counts:
 *   - Hook calls (useState, useEffect, useCallback, useMemo, etc.)
 *   - Inner function definitions assigned to variables (handlers)
 *   - Inner function declarations
 *
 * Does NOT count:
 *   - Inline JSX callbacks (onClick={() => ...})
 *   - Callbacks passed directly to hooks (useEffect(() => ..., []))
 *   - Functions defined inside nested scopes
 */

// --- Helpers ---

function isPascalCase(name) {
  return typeof name === "string" && /^[A-Z][a-zA-Z0-9]*$/.test(name);
}

function isHookCall(node) {
  if (node.type !== "CallExpression") return false;
  const callee = node.callee;
  if (callee.type === "Identifier") {
    return callee.name === "use" || /^use[A-Z]/.test(callee.name);
  }
  if (callee.type === "MemberExpression" && callee.property.type === "Identifier") {
    return callee.property.name === "use" || /^use[A-Z]/.test(callee.property.name);
  }
  return false;
}

const WRAPPER_NAMES = new Set(["forwardRef", "memo", "React.forwardRef", "React.memo"]);

function getCalleeName(callExpr) {
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
 * For arrow function or function expression nodes, check if they're
 * assigned to a PascalCase variable, optionally wrapped in forwardRef/memo.
 * Returns the component name if it is a component, null otherwise.
 */
function getComponentNameFromAssignment(node) {
  let current = node.parent;
  // Unwrap up to 2 levels of forwardRef/memo wrappers
  for (let i = 0; i < 2; i++) {
    if (current && current.type === "CallExpression") {
      const calleeName = getCalleeName(current);
      if (calleeName && WRAPPER_NAMES.has(calleeName)) {
        current = current.parent;
      } else {
        break;
      }
    }
  }
  if (
    current &&
    current.type === "VariableDeclarator" &&
    current.id &&
    current.id.type === "Identifier" &&
    isPascalCase(current.id.name)
  ) {
    return current.id.name;
  }
  return null;
}

/**
 * Find the nearest enclosing function scope for a given node.
 */
function getNearestFunctionScope(node) {
  let current = node.parent;
  while (current) {
    if (
      current.type === "FunctionDeclaration" ||
      current.type === "FunctionExpression" ||
      current.type === "ArrowFunctionExpression"
    ) {
      return current;
    }
    current = current.parent;
  }
  return null;
}

/**
 * Check if a function node is assigned to a variable
 * (indicating a named handler, not an inline callback).
 */
function isVariableAssignedFunction(node) {
  return node.parent && node.parent.type === "VariableDeclarator";
}

// --- Rule Definition ---

const maxComponentComplexity = {
  meta: {
    type: "suggestion",
    docs: {
      description: "Enforce extracting behavior logic from React components into custom hooks",
      recommended: false,
    },
    messages: {
      tooManyHooks:
        "Component '{{name}}' has {{count}} hook calls (max: {{max}}). Consider extracting behavior into a custom hook (e.g., 'use{{name}}').",
      tooManyHandlers:
        "Component '{{name}}' has {{count}} inner function definitions (max: {{max}}). Consider extracting behavior into a custom hook (e.g., 'use{{name}}').",
      tooComplex:
        "Component '{{name}}' has a behavior score of {{score}} (hooks: {{hooks}}, handlers: {{handlers}}, max: {{max}}). Extract logic into a custom hook (e.g., 'use{{name}}').",
    },
    schema: [
      {
        type: "object",
        properties: {
          maxHooks: {
            type: "number",
            minimum: 1,
          },
          maxHandlers: {
            type: "number",
            minimum: 1,
          },
          maxTotal: {
            type: "number",
            minimum: 1,
          },
        },
        additionalProperties: false,
      },
    ],
  },
  create(context) {
    const options = context.options[0] || {};
    const maxHooks = options.maxHooks ?? 5;
    const maxHandlers = options.maxHandlers ?? 3;
    const maxTotal = options.maxTotal ?? 7;

    // Stack to track component boundaries during AST traversal.
    // Supports nested components (e.g., component defined inside another).
    const componentStack = [];

    function getCurrentComponent() {
      return componentStack.length > 0 ? componentStack[componentStack.length - 1] : null;
    }

    function isDirectChildOfCurrentComponent(node) {
      const comp = getCurrentComponent();
      if (!comp) return false;
      return getNearestFunctionScope(node) === comp.node;
    }

    function pushComponent(node, name) {
      componentStack.push({ node, name, hooks: 0, handlers: 0 });
    }

    function popAndCheck(node) {
      const comp = getCurrentComponent();
      if (!comp || comp.node !== node) return;
      const info = componentStack.pop();
      const { name, hooks, handlers } = info;
      const total = hooks + handlers;

      // Report at most one message per component (most impactful first)
      if (total > maxTotal) {
        context.report({
          node,
          messageId: "tooComplex",
          data: {
            name,
            score: String(total),
            hooks: String(hooks),
            handlers: String(handlers),
            max: String(maxTotal),
          },
        });
      } else if (hooks > maxHooks) {
        context.report({
          node,
          messageId: "tooManyHooks",
          data: { name, count: String(hooks), max: String(maxHooks) },
        });
      } else if (handlers > maxHandlers) {
        context.report({
          node,
          messageId: "tooManyHandlers",
          data: { name, count: String(handlers), max: String(maxHandlers) },
        });
      }
    }

    return {
      // --- Enter/count function declarations ---
      FunctionDeclaration(node) {
        // Check if this is a React component (PascalCase name)
        if (node.id && isPascalCase(node.id.name)) {
          pushComponent(node, node.id.name);
          return;
        }
        // Otherwise, count as an inner handler if inside a component
        if (isDirectChildOfCurrentComponent(node)) {
          getCurrentComponent().handlers++;
        }
      },

      // --- Enter/count arrow function expressions ---
      ArrowFunctionExpression(node) {
        // Check if this is a React component (assigned to PascalCase var)
        const name = getComponentNameFromAssignment(node);
        if (name) {
          pushComponent(node, name);
          return;
        }
        // Count as inner handler only if assigned to a variable
        // (excludes inline JSX callbacks and hook callback arguments)
        if (isVariableAssignedFunction(node) && isDirectChildOfCurrentComponent(node)) {
          getCurrentComponent().handlers++;
        }
      },

      // --- Enter/count function expressions ---
      FunctionExpression(node) {
        // Check if this is a React component
        const name = getComponentNameFromAssignment(node);
        if (name) {
          pushComponent(node, name);
          return;
        }
        // Count as inner handler only if assigned to a variable
        if (isVariableAssignedFunction(node) && isDirectChildOfCurrentComponent(node)) {
          getCurrentComponent().handlers++;
        }
      },

      // --- Count hook calls ---
      CallExpression(node) {
        if (isHookCall(node) && isDirectChildOfCurrentComponent(node)) {
          getCurrentComponent().hooks++;
        }
      },

      // --- Exit component functions and check thresholds ---
      "FunctionDeclaration:exit"(node) {
        popAndCheck(node);
      },
      "ArrowFunctionExpression:exit"(node) {
        popAndCheck(node);
      },
      "FunctionExpression:exit"(node) {
        popAndCheck(node);
      },
    };
  },
};

// --- Plugin Export ---

const plugin = {
  meta: {
    name: "compozy-react",
  },
  rules: {
    "max-component-complexity": maxComponentComplexity,
  },
};

export default plugin;
