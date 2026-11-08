// IT-001 / IT-002: after `vite build`, the static output must hold one prerendered page per
// chapter in the registry (with its approval badge) and the not-found artifact.
// Runs as the tail of `bun run build`, so CI's build lane fails on a missing or broken page.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { hrefOf, pages } from "../src/manual/chapters";

const OUT = join(import.meta.dirname, "..", ".output", "public");
const failures: string[] = [];

for (const page of pages) {
  const href = hrefOf(page);
  const file = join(OUT, href === "/" ? "index.html" : `${href.slice(1)}/index.html`);
  if (!existsSync(file)) {
    failures.push(`missing prerendered page for ${href} (${file})`);
    continue;
  }
  const html = readFileSync(file, "utf8");
  if (!html.includes(`data-status="${page.status}"`)) {
    failures.push(`${href} does not render its "${page.status}" status badge`);
  }
  for (const [id] of page.sections) {
    if (!html.includes(`id="${id}"`)) failures.push(`${href} is missing section anchor #${id}`);
  }
}

const notFound = join(OUT, "404.html");
if (!existsSync(notFound)) failures.push(`missing not-found artifact (${notFound})`);
else if (!readFileSync(notFound, "utf8").includes("data-not-found")) {
  failures.push("404.html does not render the not-found page");
}

if (failures.length > 0) {
  console.error(`build output check failed:\n- ${failures.join("\n- ")}`);
  process.exit(1);
}
console.log(`build output check: ${pages.length} chapters + 404.html prerendered in ${OUT}`);
