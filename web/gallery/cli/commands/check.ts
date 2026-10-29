import { resolve } from "node:path";
import { build } from "vite";

import type { Problem } from "../../node/discovery.ts";
import { galleryRoot } from "../../node/paths.ts";
import type { ParsedArgs } from "../args.ts";
import { discoverFlows } from "../registry.ts";

/**
 * `proto check [--json]`: prototype and flow discovery problems, then a gallery build that writes
 * nothing, which runs the network guard over every prototype's import graph. Flow coverage
 * warnings are reported but never fail the check.
 */
export async function check(args: ParsedArgs) {
  const json = args.flags.json === true;
  const result = discoverFlows();
  const problems: Problem[] = [...result.problems];
  const { flowProblems, warnings } = result;
  let buildError: string | null = null;
  if (problems.length === 0 && flowProblems.length === 0) {
    try {
      await build({
        configFile: resolve(galleryRoot, "vite.config.ts"),
        build: { write: false },
        logLevel: json ? "silent" : "error",
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (message.includes("api_client:")) {
        const entry = result.entries.find(item =>
          message.includes(item.file.replace(/^web\//, ""))
        );
        problems.push({
          code: "api_client",
          slug: entry?.slug ?? null,
          file: entry?.file ?? "",
          message,
        });
      } else {
        buildError = message;
      }
    }
  }
  const ok = problems.length === 0 && flowProblems.length === 0 && buildError === null;
  if (json) {
    console.log(
      JSON.stringify({
        ok,
        prototypes: result.entries.length,
        specs: result.catalog.available ? result.catalog.specs.length : null,
        flows: result.flows.length,
        problems,
        flowProblems,
        warnings,
        ...(buildError ? { error: buildError } : {}),
      })
    );
  } else if (buildError) {
    console.error(buildError);
  } else if (problems.length > 0 || flowProblems.length > 0) {
    console.error(
      [...problems, ...flowProblems]
        .map(problem => `${problem.file}: ${problem.message}`)
        .join("\n")
    );
  } else {
    const specs = result.catalog.available
      ? `${result.catalog.specs.length} specs`
      : "spec validation skipped (no .compozy/tasks)";
    console.log(
      `${result.entries.length} prototypes valid · ${result.flows.length} flows valid · ${specs}`
    );
    if (warnings.length > 0) {
      const slugs = new Set(warnings.map(warning => warning.slug)).size;
      console.warn(
        `${warnings.length} prototype states in ${slugs} prototypes are in no flow (bun run proto flow coverage)`
      );
    }
  }
  if (!ok) process.exitCode = 1;
}
