import { relative, resolve } from "node:path";

/**
 * The daemon network layer prototypes and drafts must never reach at runtime: the API client,
 * ticketed SSE and WebSocket transports, gateway stream auth and TanStack Query. Prototypes render
 * views from fixtures; data access belongs to the route or hook that owns it.
 */
const networkModules = ["api-client.ts", "ticketed-event-source.ts", "ticketed-web-socket.ts"];
const gatewayModule = /[\\/]src[\\/]lib[\\/]gateway-[^\\/]+\.ts$/;
export const networkPackagePattern =
  /(?:^|[\\/]node_modules[\\/])@tanstack[\\/]react-query(?:[\\/]|$)/;
export const networkPackageSpecifier = /^@tanstack\/react-query(?:\/|$)/;

const prototypeModule = /\.proto\.[cm]?[jt]sx?$|[\\/]src[\\/]drafts[\\/]/;

export function isPrototypeModule(id: string) {
  return prototypeModule.test(id);
}

export function networkModuleMatcher(webRoot: string) {
  const forbidden = new Set(networkModules.map(name => resolve(webRoot, "src/lib", name)));
  return (id: string) =>
    forbidden.has(id) ||
    (id.startsWith(resolve(webRoot, "src/lib")) && gatewayModule.test(id)) ||
    networkPackagePattern.test(id) ||
    networkPackageSpecifier.test(id);
}

export type ImportGraph = ReadonlyMap<string, ReadonlySet<string>>;

/** The first import chain from `id` to a forbidden module, depth first; undefined when none. */
export function forbiddenChain(
  id: string,
  graph: ImportGraph,
  forbidden: (id: string) => boolean,
  seen = new Set<string>()
): string[] | undefined {
  if (forbidden(id)) return [id];
  if (seen.has(id)) return undefined;
  seen.add(id);
  for (const dependency of graph.get(id) ?? []) {
    const chain = forbiddenChain(dependency, graph, forbidden, seen);
    if (chain) return [id, ...chain];
  }
  return undefined;
}

export function violationMessage(chain: readonly string[], webRoot: string) {
  const shown = chain.map(id => (id.startsWith("/") ? relative(webRoot, id) : id));
  return `api_client: ${shown[0]} reaches the daemon network layer: ${shown.join(" → ")}. Prototypes render views from fixtures; move data access to the route or hook that owns it.`;
}
