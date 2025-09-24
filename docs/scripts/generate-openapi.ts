import { generateFiles } from "fumadocs-openapi";
import { rm, writeFile } from "node:fs/promises";
import path from "node:path";
import { loadOpenAPIDocument } from "../src/lib/openapi-loader";

async function main(): Promise<void> {
  const document = await loadOpenAPIDocument();
  const patchedPath = path.join(process.cwd(), "swagger.patched.json");
  await writeFile(patchedPath, JSON.stringify(document, null, 2), "utf8");
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
