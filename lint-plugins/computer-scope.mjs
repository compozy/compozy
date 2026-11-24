/**
 * OXC/ESLint Plugin: Compozy computer-scoped query keys (ADR-004, UT-092).
 *
 * Every TanStack Query key in `web/src/systems/*` (outside controller-only
 * `account`, `computers`, and `gateway` systems) must begin with
 * `["computer", computerId, ...]` or compose a computer-scoped key factory.
 */

const EXEMPT_PATH_SEGMENTS = ["/__tests__/", "/stories/", "/.storybook/", "/drafts/"];
const EXEMPT_FILE_SUFFIXES = [
  ".test.tsx",
  ".test.ts",
  ".spec.tsx",
  ".spec.ts",
  ".stories.tsx",
  ".stories.ts",
  ".proto.tsx",
  ".proto.ts",
];
const CONTROLLER_ONLY_SYSTEMS = new Set(["account", "computers", "gateway"]);
const QUERY_FUNCTIONS = new Set([
  "useQuery",
  "useInfiniteQuery",
  "useSuspenseQuery",
  "useSuspenseInfiniteQuery",
  "queryOptions",
  "infiniteQueryOptions",
]);
const QUERY_CLIENT_METHODS = new Set([
  "ensureQueryData",
  "ensureInfiniteQueryData",
  "fetchQuery",
  "fetchInfiniteQuery",
  "prefetchQuery",
  "prefetchInfiniteQuery",
  "cancelQueries",
  "invalidateQueries",
  "refetchQueries",
  "removeQueries",
  "resetQueries",
]);

function normalizeFilename(filename) {
  return typeof filename === "string" ? filename.replaceAll("\\", "/") : "";
}

function systemNameFromPath(normalized) {
  const marker = "/web/src/systems/";
  const index = normalized.indexOf(marker);
  if (index === -1) return null;
  const rest = normalized.slice(index + marker.length);
  const slash = rest.indexOf("/");
  return slash === -1 ? null : rest.slice(0, slash);
}

function isCheckedPath(filename) {
  const normalized = normalizeFilename(filename);
  const system = systemNameFromPath(normalized);
  if (!system || CONTROLLER_ONLY_SYSTEMS.has(system)) return false;
  if (EXEMPT_PATH_SEGMENTS.some(segment => normalized.includes(segment))) return false;
  if (EXEMPT_FILE_SUFFIXES.some(suffix => normalized.endsWith(suffix))) return false;
  return (
    normalized.includes("/hooks/") ||
    normalized.includes("/lib/") ||
    normalized.includes("/components/") ||
    normalized.includes("/routes/")
  );
}

function isQueryCall(callee, queryFunctions) {
  if (callee?.type === "Identifier" && queryFunctions.has(callee.name)) {
    return true;
  }
  if (callee?.type !== "MemberExpression" || callee.computed) return false;
  if (callee.property?.type !== "Identifier") return false;
  return QUERY_CLIENT_METHODS.has(callee.property.name);
}

function collectQueryImports(node, queryFunctions) {
  if (node.source?.value !== "@tanstack/react-query") return;
  for (const specifier of node.specifiers ?? []) {
    if (specifier.type !== "ImportSpecifier") continue;
    const importedName = specifier.imported?.name ?? specifier.imported?.value;
    if (QUERY_FUNCTIONS.has(importedName) && specifier.local?.name) {
      queryFunctions.add(specifier.local.name);
    }
  }
}

function propertyName(property) {
  if (property?.type !== "Property" || property.computed) return null;
  if (property.key.type === "Identifier") return property.key.name;
  return property.key.type === "Literal" && typeof property.key.value === "string"
    ? property.key.value
    : null;
}

function findProperty(objectExpression, name) {
  return objectExpression.properties.find(property => propertyName(property) === name);
}

function unwrapExpression(node) {
  if (!node) return null;
  if (node.type === "TSAsExpression" || node.type === "TSSatisfiesExpression") {
    return unwrapExpression(node.expression);
  }
  return node;
}

function isComputerScopedArray(arrayExpression) {
  const elements = arrayExpression.elements ?? [];
  if (elements.length === 0) return false;
  const first = unwrapExpression(elements[0]);
  if (!first) return false;
  // Spread of an existing computer-scoped factory, e.g. `[...sessionKeys.all, "detail"]`.
  if (first.type === "SpreadElement") return true;
  if (first.type === "Literal" && first.value === "computer") {
    return elements.length >= 2 && elements[1] != null;
  }
  return false;
}

const queryKeyComputerScope = {
  meta: {
    type: "problem",
    docs: {
      description:
        'Require TanStack Query keys in web/src/systems/* to be computer-scoped (["computer", computerId, ...]).',
    },
    messages: {
      missingComputerScope:
        'Query keys must be computer-scoped: start with ["computer", computerId, ...] or compose a computer-scoped key factory.',
    },
    schema: [],
  },
  create(context) {
    if (!isCheckedPath(context.filename || "")) return {};
    const queryFunctions = new Set(QUERY_FUNCTIONS);
    return {
      ImportDeclaration(node) {
        collectQueryImports(node, queryFunctions);
      },
      CallExpression(node) {
        if (!isQueryCall(node.callee, queryFunctions)) return;
        const options = unwrapExpression(node.arguments?.[0]);
        if (options?.type !== "ObjectExpression") return;
        const queryKeyProp = findProperty(options, "queryKey");
        if (!queryKeyProp) return;
        const keyValue = unwrapExpression(queryKeyProp.value);
        if (keyValue?.type !== "ArrayExpression") return;
        if (!isComputerScopedArray(keyValue)) {
          context.report({ node: keyValue, messageId: "missingComputerScope" });
        }
      },
    };
  },
};

const plugin = {
  meta: { name: "compozy" },
  rules: {
    "query-key-computer-scope": queryKeyComputerScope,
  },
};

export default plugin;
