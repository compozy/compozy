// IT-001 / IT-002: after `vite build`, the static output must hold one prerendered page per
// chapter in the registry (with its approval badge), one /kit/<id> page per kit template (with
// its [data-kit] root), /kit/index.json listing every template, and the not-found artifact.
// Runs as the tail of `bun run build`, so CI's build lane fails on a missing or broken page.
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { hrefOf, pages } from "../src/manual/chapters";
import { kitIds } from "../src/manual/templates/registry";

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

const kit = kitIds();
for (const id of kit) {
  const file = join(OUT, "kit", id, "index.html");
  if (!existsSync(file)) failures.push(`missing prerendered kit page /kit/${id} (${file})`);
  else if (!readFileSync(file, "utf8").includes(`data-kit="${id}"`)) {
    failures.push(`/kit/${id} does not render its [data-kit] root`);
  }
}
const indexFile = join(OUT, "kit", "index.json");
if (!existsSync(indexFile)) failures.push(`missing /kit/index.json (${indexFile})`);
else {
  const listed = new Set(
    (JSON.parse(readFileSync(indexFile, "utf8")) as { id: string }[]).map(r => r.id)
  );
  const unlisted = kit.filter(id => !listed.has(id));
  if (unlisted.length > 0) failures.push(`/kit/index.json misses ${unlisted.join(", ")}`);
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
console.log(
  `build output check: ${pages.length} chapters, ${kit.length} kit templates, kit/index.json and 404.html prerendered in ${OUT}`
);
