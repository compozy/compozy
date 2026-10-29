/**
 * `proto flow new|list|show|coverage|url`: author and inspect journey flows. Every read resolves
 * flows against current prototype discovery, so the output matches what the gallery draws.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { relative, resolve } from "node:path";

import { membership } from "../../flow-graph.ts";
import type { FlowEntry } from "../../flow.ts";
import { flowListRow, flowShowJson } from "../../node/flow-output.ts";
import { validFlowId } from "../../node/flows.ts";
import { galleryOrigin, galleryRoot, repoRoot } from "../../node/paths.ts";
import { stringFlag, UsageError, type ParsedArgs } from "../args.ts";
import { discoverFlows } from "../registry.ts";

const subcommands = ["new", "list", "show", "coverage", "url"] as const;

function knownSpec(spec: string | undefined, registry: ReturnType<typeof discoverFlows>) {
  const { catalog } = registry;
  if (spec && catalog.available && !catalog.specs.some(item => item.id === spec)) {
    throw new UsageError(`Unknown spec "${spec}" (no .compozy/tasks/${spec}/)`);
  }
}

function findFlow(id: string | undefined): FlowEntry {
  if (!id) throw new UsageError("Missing <id>");
  const registry = discoverFlows();
  const found = registry.flows.find(flow => flow.id === id);
  if (found) return found;
  const problems = registry.flowProblems.filter(problem => problem.flow === id);
  throw new Error(
    problems.length > 0
      ? problems.map(problem => `${problem.file}: ${problem.message}`).join("\n")
      : `Unknown flow "${id}" (bun run proto flow list)`
  );
}

function create(id: string | undefined, args: ParsedArgs) {
  if (!id || !validFlowId(id)) throw new UsageError(`Invalid flow id "${id ?? ""}"`);
  const spec = stringFlag(args, "spec");
  knownSpec(spec, discoverFlows());
  const folder = resolve(galleryRoot, "flows");
  mkdirSync(folder, { recursive: true });
  const file = resolve(folder, `${id}.flow.ts`);
  const flow = {
    title: stringFlag(args, "title") ?? "New flow",
    ...(spec ? { spec } : {}),
    start: ["docs"],
    nodes: { docs: { external: "docs", end: true } },
    edges: [],
  };
  try {
    writeFileSync(
      file,
      `import { defineFlow } from "#gallery/flow";\n\nexport const flow = defineFlow(${JSON.stringify(flow, null, 2)});\n`,
      { flag: "wx" }
    );
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "EEXIST") {
      throw new Error(`${relative(repoRoot, file)} already exists`);
    }
    throw error;
  }
  console.log(`Created ${relative(repoRoot, file)}`);
}

function list(args: ParsedArgs) {
  const registry = discoverFlows();
  const spec = stringFlag(args, "spec");
  knownSpec(spec, registry);
  const rows = registry.flows.filter(flow => !spec || flow.specs.includes(spec)).map(flowListRow);
  if (args.flags.json === true) {
    console.log(JSON.stringify(rows));
    return;
  }
  for (const row of rows) {
    console.log(
      [
        row.id,
        row.spec ?? "-",
        `${row.specs.length} specs`,
        `${row.nodes} nodes`,
        `${row.edges} edges`,
        row.file,
      ].join("\t")
    );
  }
}

function show(id: string | undefined, args: ParsedArgs) {
  const flow = findFlow(id);
  if (args.flags.json === true) {
    console.log(JSON.stringify(flowShowJson(flow)));
    return;
  }
  const label = (node: string) => {
    const found = flow.nodes.find(item => item.id === node);
    if (!found) return node;
    return found.kind === "prototype" ? `${found.title} (${found.state})` : found.label;
  };
  console.log(`${flow.title} · ${flow.id} · start: ${flow.start.join(", ")}`);
  for (const edge of flow.edges) {
    console.log(`${label(edge.from)} → ${label(edge.to)} · "${edge.on}" · ${edge.kind}`);
  }
}

function coverageRows(args: ParsedArgs) {
  const registry = discoverFlows();
  const spec = stringFlag(args, "spec");
  knownSpec(spec, registry);
  const used = membership(registry.flows);
  return registry.entries.flatMap(prototype => {
    const meta = prototype.meta;
    if (!meta || (spec && meta.spec !== spec)) return [];
    const states = meta.states.map(state => ({
      id: state.id,
      flows: [...new Set((used.get(`${prototype.slug}:${state.id}`) ?? []).map(item => item.flow))],
    }));
    return [{ slug: prototype.slug, spec: meta.spec ?? null, states }];
  });
}

function coverage(args: ParsedArgs) {
  const rows = coverageRows(args);
  if (args.flags.json === true) {
    console.log(JSON.stringify(rows));
    return;
  }
  for (const row of rows) {
    const missing = row.states.flatMap(state => (state.flows.length === 0 ? [state.id] : []));
    const covered = row.states.length - missing.length;
    console.log(
      `${row.slug}\t${covered}/${row.states.length}${missing.length > 0 ? `\tmissing: ${missing.join(", ")}` : ""}`
    );
  }
}

function url(id: string | undefined, args: ParsedArgs) {
  const flow = findFlow(id);
  const node = stringFlag(args, "node");
  if (node && !flow.nodes.some(item => item.id === node)) {
    throw new UsageError(
      `Unknown node "${node}" in flow "${flow.id}": ${flow.nodes.map(item => item.id).join(", ")}`
    );
  }
  const target = new URL("/", galleryOrigin);
  target.searchParams.set("flow", flow.id);
  if (node) target.searchParams.set("node", node);
  console.log(target.href);
}

/** `proto flow <new|list|show|coverage|url> [<id>] …`. */
export function flowCommand(args: ParsedArgs) {
  const [name, id] = args.positionals;
  switch (name) {
    case "new":
      return create(id, args);
    case "list":
      return list(args);
    case "show":
      return show(id, args);
    case "coverage":
      return coverage(args);
    case "url":
      return url(id, args);
    default:
      throw new UsageError(`Unknown flow command "${name ?? ""}" (${subcommands.join(", ")})`);
  }
}
