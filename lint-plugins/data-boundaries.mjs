/**
 * OXC/ESLint Plugin: AGH Web data boundaries.
 *
 * Server-query consumers reuse canonical option and key factories. Query
 * definitions belong in system `lib/query-options.ts`; query-key composition
 * belongs in `lib/query-keys.ts`.
 */

const EXEMPT_PATH_SEGMENTS = ["/__tests__/", "/stories/", "/.storybook/"];
const EXEMPT_FILE_SUFFIXES = [
  ".test.tsx",
  ".test.ts",
  ".spec.tsx",
  ".spec.ts",
  ".stories.tsx",
  ".stories.ts",
];
const CONSUMER_PATH_SEGMENTS = ["/routes/", "/hooks/", "/components/"];
const SERVER_QUERY_FUNCTIONS = new Set([
  "useQuery",
  "useInfiniteQuery",
  "useSuspenseQuery",
  "useSuspenseInfiniteQuery",
]);
const SERVER_QUERY_METHODS = new Set([
  "ensureQueryData",
  "ensureInfiniteQueryData",
  "fetchQuery",
  "fetchInfiniteQuery",
  "prefetchQuery",
  "prefetchInfiniteQuery",
]);
const QUERY_KEY_METHODS = new Set([
  ...SERVER_QUERY_METHODS,
  "cancelQueries",
  "invalidateQueries",
  "refetchQueries",
  "removeQueries",
  "resetQueries",
]);

function normalizeFilename(filename) {
  return typeof filename === "string" ? filename.replaceAll("\\", "/") : "";
}

function isConsumerSourcePath(filename) {
  const normalized = normalizeFilename(filename);
  if (!normalized.includes("/web/src/")) return false;
  if (!CONSUMER_PATH_SEGMENTS.some(segment => normalized.includes(segment))) return false;
  if (EXEMPT_PATH_SEGMENTS.some(segment => normalized.includes(segment))) return false;
  return !EXEMPT_FILE_SUFFIXES.some(suffix => normalized.endsWith(suffix));
}

function calleeKind(callee, serverQueryFunctions = SERVER_QUERY_FUNCTIONS) {
  if (callee?.type === "Identifier" && serverQueryFunctions.has(callee.name)) {
    return "server-query";
  }
  if (callee?.type !== "MemberExpression" || callee.computed) return null;
  if (callee.property?.type !== "Identifier") return null;
  if (SERVER_QUERY_METHODS.has(callee.property.name)) return "server-query";
  if (QUERY_KEY_METHODS.has(callee.property.name)) return "query-key";
  return null;
}

function collectServerQueryImport(node, serverQueryFunctions) {
  if (node.source?.value !== "@tanstack/react-query") return;
  for (const specifier of node.specifiers ?? []) {
    if (specifier.type !== "ImportSpecifier") continue;
    const imported = specifier.imported;
    const importedName = imported?.name ?? imported?.value;
    if (SERVER_QUERY_FUNCTIONS.has(importedName) && specifier.local?.name) {
      serverQueryFunctions.add(specifier.local.name);
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

const requireCanonicalQueryOptions = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Require Web query consumers to reuse a canonical option factory instead of defining queryKey/queryFn inline.",
    },
    messages: {
      inlineDefinition:
        "Move queryKey/queryFn into the system's lib/query-options.ts factory and pass or spread that factory here. Loaders, hooks, mutations, and streams must share one server-query identity.",
    },
    schema: [],
  },
  create(context) {
    if (!isConsumerSourcePath(context.filename || "")) return {};
    const serverQueryFunctions = new Set(SERVER_QUERY_FUNCTIONS);
    return {
      ImportDeclaration(node) {
        collectServerQueryImport(node, serverQueryFunctions);
      },
      CallExpression(node) {
        if (calleeKind(node.callee, serverQueryFunctions) !== "server-query") return;
        const options = node.arguments?.[0];
        if (options?.type !== "ObjectExpression") return;
        const queryKey = findProperty(options, "queryKey");
        const queryFn = findProperty(options, "queryFn");
        if (!queryKey && !queryFn) return;
        context.report({
          node: queryKey ?? queryFn ?? options,
          messageId: "inlineDefinition",
        });
      },
    };
  },
};

const noInlineQueryKey = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Forbid inline query-key arrays in Web consumers so cache reads and writes share key factories.",
    },
    messages: {
      inlineKey:
        "Use the system's named query-key factory instead of composing a queryKey array at this consumer.",
    },
    schema: [],
  },
  create(context) {
    if (!isConsumerSourcePath(context.filename || "")) return {};
    const serverQueryFunctions = new Set(SERVER_QUERY_FUNCTIONS);
    return {
      ImportDeclaration(node) {
        collectServerQueryImport(node, serverQueryFunctions);
      },
      CallExpression(node) {
        const kind = calleeKind(node.callee, serverQueryFunctions);
        if (!kind) return;
        const options = node.arguments?.[0];
        if (options?.type !== "ObjectExpression") return;
        const queryKey = findProperty(options, "queryKey");
        if (queryKey?.value?.type !== "ArrayExpression") return;
        context.report({ node: queryKey.value, messageId: "inlineKey" });
      },
    };
  },
};

const plugin = {
  meta: { name: "compozy-data-boundaries" },
  rules: {
    "require-canonical-query-options": requireCanonicalQueryOptions,
    "no-inline-query-key": noInlineQueryKey,
  },
};

export default plugin;
