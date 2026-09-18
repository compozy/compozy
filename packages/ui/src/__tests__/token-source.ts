import { readFileSync } from "node:fs";
import { join } from "node:path";

// Real token bytes, so contract suites audit what ships rather than a copy.
const TOKENS_CSS = readFileSync(join(__dirname, "..", "tokens.css"), "utf8");

export function readToken(name: string): string {
  const match = TOKENS_CSS.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  if (!match) throw new Error(`token --${name} not found in tokens.css`);
  return match[1].replace(/\s+/g, " ").trim();
}
