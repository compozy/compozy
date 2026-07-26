import { fileURLToPath } from "node:url";
import path from "node:path";
import { createOpenAPI } from "fumadocs-openapi/server";

const HERE = path.dirname(fileURLToPath(import.meta.url));

export const AGH_OPENAPI_ID = "agh";
export const AGH_OPENAPI_PATH = path.resolve(HERE, "../../../openapi/agh.json");

export const openapi = createOpenAPI({
  input: { [AGH_OPENAPI_ID]: AGH_OPENAPI_PATH },
});
