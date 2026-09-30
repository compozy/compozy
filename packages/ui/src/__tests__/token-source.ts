import { readFileSync } from "node:fs";
import { join } from "node:path";

// Real token bytes, so contract suites audit what ships rather than a copy.
export const TOKENS_CSS = readFileSync(join(__dirname, "..", "tokens.css"), "utf8");
export const TOKENS_LIGHT_CSS = readFileSync(join(__dirname, "..", "tokens-light.css"), "utf8");

const ADAPTER = /^var\(--([a-zA-Z0-9-]+)\)$/;

function readRaw(name: string): string {
  const match = TOKENS_CSS.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  if (!match) throw new Error(`token --${name} not found in tokens.css`);
  return match[1].replace(/\s+/g, " ").trim();
}

/** The default (dark) value of a token, following single-`var()` adapters to their literal. */
export function readToken(name: string): string {
  const seen = new Set<string>();
  let current = name;
  let value = readRaw(current);
  for (let adapter = value.match(ADAPTER); adapter; adapter = value.match(ADAPTER)) {
    if (seen.has(current)) throw new Error(`token --${name} has a var() cycle`);
    seen.add(current);
    current = adapter[1];
    value = readRaw(current);
  }
  return value;
}
