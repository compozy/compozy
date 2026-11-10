/** Facts about a generated API resource page, read from its raw MDX source. */

/** The `operations` the generator passed to `<OpenAPIPage>`, each one `{"path":…,"method":…}`. */
const OPERATION = /\{"path":"[^"]+","method":"[a-z]+"\}/g;

/** How many endpoints a generated API page documents; 0 for every other page. */
export function apiOperationCount(source: string): number {
  if (!source.includes("<OpenAPIPage")) return 0;
  return source.match(OPERATION)?.length ?? 0;
}
