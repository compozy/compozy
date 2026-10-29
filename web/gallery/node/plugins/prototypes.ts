import { resolve } from "node:path";
import type { Plugin } from "vite";

import { findPrototypes } from "../discovery.ts";
import { findFlows } from "../flows.ts";
import { findSpecs } from "../specs.ts";

export const registryModuleId = "virtual:compozy-prototypes";

function discover(webRoot: string, repoRoot: string) {
  const catalog = findSpecs(repoRoot);
  const prototypes = findPrototypes(webRoot, repoRoot, catalog);
  return {
    catalog,
    ...prototypes,
    ...flowsOf(findFlows(webRoot, repoRoot, prototypes.entries, catalog)),
  };
}

function flowsOf(result: ReturnType<typeof findFlows>) {
  return { flows: result.flows, flowProblems: result.problems, warnings: result.warnings };
}

/**
 * Serves `virtual:compozy-prototypes`: every discovered prototype with a lazy loader, the spec
 * catalog, the journey flows with their coverage warnings, and discovery problems. New, removed
 * or edited prototypes and flows rebuild the module and reload the gallery; a build with
 * problems fails before bundling (coverage warnings never fail it).
 */
export function prototypeRegistry(webRoot: string, repoRoot: string): Plugin {
  const id = `\0${registryModuleId}`;
  let building = false;
  return {
    name: "compozy:prototypes",
    config() {
      // macOS native watchers can register paths without delivering events in managed hosts;
      // polling lets discovery see new prototype files too.
      if (process.platform === "darwin") return { server: { watch: { usePolling: true } } };
      return undefined;
    },
    configResolved(config) {
      building = config.command === "build";
    },
    resolveId(source) {
      return source === registryModuleId ? id : undefined;
    },
    load(source) {
      if (source !== id) return undefined;
      const { catalog, entries, problems, flows, flowProblems, warnings } = discover(
        webRoot,
        repoRoot
      );
      const loaders = entries.map(
        entry =>
          `{...${JSON.stringify(entry)}, load: () => import(${JSON.stringify(resolve(repoRoot, entry.file))})}`
      );
      return `export const specs = ${JSON.stringify(catalog.specs)};
export const problems = ${JSON.stringify(problems)};
export const flows = ${JSON.stringify(flows)};
export const flowProblems = ${JSON.stringify(flowProblems)};
export const warnings = ${JSON.stringify(warnings)};
export const prototypes = [${loaders.join(",")}];`;
    },
    buildStart() {
      if (!building) return;
      const { problems, flowProblems } = discover(webRoot, repoRoot);
      const all = [...problems, ...flowProblems];
      if (all.length > 0) {
        this.error(all.map(problem => `${problem.file}: ${problem.message}`).join("\n"));
      }
    },
    configureServer(server) {
      // Prototypes live outside the gallery root, including files nothing imports yet; flows are
      // never imported at all (discovery reads them).
      server.watcher.add([resolve(webRoot, "src"), resolve(webRoot, "gallery/flows")]);
      const refresh = (path: string) => {
        if (!path.endsWith(".proto.tsx") && !path.endsWith(".flow.ts")) return;
        const module = server.moduleGraph.getModuleById(id);
        if (module) server.moduleGraph.invalidateModule(module);
        server.ws.send({ type: "full-reload" });
      };
      server.watcher.on("add", refresh).on("unlink", refresh).on("change", refresh);
    },
  };
}
