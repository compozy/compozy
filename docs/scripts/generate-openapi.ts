import { generateFiles } from "fumadocs-openapi";
import { randomUUID } from "node:crypto";
import { readdir, rm } from "node:fs/promises";
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
    await rewriteDocumentPaths(patchedPath);
  } finally {
    await rm(patchedPath, { force: true });
  }
}

async function rewriteDocumentPaths(patchedPath: string): Promise<void> {
  const directory = path.join(process.cwd(), "content/docs/api");
  const entries = await listMdxFiles(directory);
  const tokens = new Set<string>();
  tokens.add(`{${JSON.stringify(patchedPath)}}`);
  const relativePatchedPath = path.relative(process.cwd(), patchedPath);
  tokens.add(`{${JSON.stringify(relativePatchedPath)}}`);
  const replacementToken = '{"swagger.json"}';
  await Promise.all(
    entries.map(async filePath => {
      const contents = await Bun.file(filePath).text();
      let updated = contents;
      for (const token of tokens) {
        if (!updated.includes(token)) {
          continue;
        }
        updated = updated.replaceAll(token, replacementToken);
      }
      if (updated === contents) {
        return;
      }
      await Bun.write(filePath, updated);
    })
  );
}

async function listMdxFiles(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const files: string[] = [];
  await Promise.all(
    entries.map(async entry => {
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        files.push(...(await listMdxFiles(fullPath)));
        return;
      }
      if (entry.isFile() && entry.name.endsWith(".mdx")) {
        files.push(fullPath);
      }
    })
  );
  return files;
}

void main();
