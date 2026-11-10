/** Facts about a generated API resource page, read from its raw MDX source. */

/** The `operations` the generator passed to `<OpenAPIPage>`, each one `{"path":…,"method":…}`. */
const OPERATION = /\{"path":"([^"]+)","method":"([a-z]+)"\}/g;

/** The endpoints a generated API page documents, in page order; none for every other page. */
export function apiOperations(source: string): { method: string; path: string }[] {
  if (!source.includes("<OpenAPIPage")) return [];
  return [...source.matchAll(OPERATION)].map(([, path = "", method = ""]) => ({ method, path }));
}

/** How many endpoints a generated API page documents; 0 for every other page. */
export function apiOperationCount(source: string): number {
  return apiOperations(source).length;
}

/**
 * The anchor of one endpoint section. Summaries repeat within a resource ("List sessions" is both
 * the global and the workspace route), so the anchor comes from the method and path instead.
 */
export function operationAnchor(method: string, path: string): string {
  const slug = `${method}-${path}`
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return `op-${slug}`;
}
