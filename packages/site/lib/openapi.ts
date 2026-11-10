import { fileURLToPath } from "node:url";
import fs from "node:fs/promises";
import path from "node:path";
import { createOpenAPI } from "fumadocs-openapi/server";

const HERE = path.dirname(fileURLToPath(import.meta.url));

export const COMPOZY_OPENAPI_ID = "compozy";
export const COMPOZY_OPENAPI_PATH = path.resolve(HERE, "../../../openapi/compozy.json");

/** Where a default install serves the HTTP API (`[http]` host and port in `config.toml`). */
export const COMPOZY_API_DEFAULT_SERVER = "http://localhost:2123";

/**
 * The generated contract names no server, so request samples would fall back to a placeholder host.
 * The reference points them at the local daemon instead.
 */
async function loadDocument() {
  const document = JSON.parse(await fs.readFile(COMPOZY_OPENAPI_PATH, "utf8"));
  return { servers: [{ url: COMPOZY_API_DEFAULT_SERVER }], ...document };
}

export const openapi = createOpenAPI({
  input: { [COMPOZY_OPENAPI_ID]: loadDocument },
});
