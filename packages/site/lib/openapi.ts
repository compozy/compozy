import { fileURLToPath } from "node:url";
import path from "node:path";
import { createOpenAPI } from "fumadocs-openapi/server";

const HERE = path.dirname(fileURLToPath(import.meta.url));

export const COMPOZY_OPENAPI_ID = "compozy";
export const COMPOZY_OPENAPI_PATH = path.resolve(HERE, "../../../openapi/compozy.json");

export const openapi = createOpenAPI({
  input: { [COMPOZY_OPENAPI_ID]: COMPOZY_OPENAPI_PATH },
});
