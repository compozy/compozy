#!/usr/bin/env node
/**
 * Generate the TanStack Router route tree (`web/src/routeTree.gen.ts`).
 *
 * The Vite plugin handles this at dev/build time, but typecheck and vitest
 * need the file before any compilation happens. Run this before those tasks.
 */

import { Generator, getConfig } from "@tanstack/router-generator";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(here, "..");

const config = await getConfig({}, webRoot);
const generator = new Generator({ config, root: webRoot });
await generator.run();
