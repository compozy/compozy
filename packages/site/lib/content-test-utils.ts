import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
export const contentRoot = resolve(siteRoot, "content");
