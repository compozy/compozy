import { randomUUID } from "node:crypto";

import { Compozy as BrowserCompozy } from "./client.js";
import { discover } from "./node-discovery.js";

export * from "./index.js";

/** Node, Bun, and Deno entry point with filesystem discovery. */
export class Compozy extends BrowserCompozy {
  protected static override discovery = discover;
  protected override createPromptId = randomUUID;
}
