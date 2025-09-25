import { generateFiles } from "fumadocs-openapi";
import { randomUUID } from "node:crypto";
import { rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { loadOpenAPIDocument } from "../src/lib/openapi-loader";

async function main(): Promise<void> {
  const document = await loadOpenAPIDocument();
  const patchedPath = path.join(tmpdir(), `swagger.patched.${randomUUID()}.json`);
  await Bun.write(patchedPath, JSON.stringify(document, null, 2));
  try {
    await generateFiles({
      input: [patchedPath],
      output: "./content/docs/api",
      per: "tag",
      includeDescription: true,
      addGeneratedComment:
        "<!-- This file was auto-generated from OpenAPI/Swagger. Do not edit manually. -->",
      frontmatter: (title, description) => ({
        title,
        description,
        icon: "Code",
        group: "API Reference",
      }),
    });
  } finally {
    await rm(patchedPath, { force: true });
  }
}

void main();
