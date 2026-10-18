import { readFileSync } from "node:fs";
import { join } from "node:path";

// Real token bytes, so contract suites audit what ships rather than a copy.
export const TOKENS_CSS = readFileSync(join(__dirname, "..", "tokens.css"), "utf8");
export const TOKENS_LIGHT_CSS = readFileSync(join(__dirname, "..", "tokens-light.css"), "utf8");

export type Theme = "dark" | "light";
export const THEMES: readonly Theme[] = ["dark", "light"];

const ADAPTER = /^var\(--([a-zA-Z0-9-]+)\)$/;

function lightBlock(): string {
  const match = TOKENS_LIGHT_CSS.match(/\[data-theme="light"\]:root\s*\{([\s\S]*?)\n\}/);
  if (!match) throw new Error('no [data-theme="light"]:root block in tokens-light.css');
  return match[1];
}

const LIGHT_BLOCK = lightBlock();

function firstDeclaration(css: string, name: string): string | undefined {
  const match = css.match(new RegExp(`--${name}\\s*:\\s*([^;]+);`));
  return match?.[1].replace(/\s+/g, " ").trim();
}

function readRaw(name: string, theme: Theme): string {
  const value =
    (theme === "light" ? firstDeclaration(LIGHT_BLOCK, name) : undefined) ??
    firstDeclaration(TOKENS_CSS, name);
  if (value === undefined) throw new Error(`token --${name} not found in tokens.css`);
  return value;
}

/**
 * A token's value in one theme (dark, the default, unless asked), following
 * single-`var()` adapters to their literal. Light reads the tokens-light.css
 * override and falls back to the shared declaration.
 */
export function readToken(name: string, theme: Theme = "dark"): string {
  const seen = new Set<string>();
  let current = name;
  let value = readRaw(current, theme);
  for (let adapter = value.match(ADAPTER); adapter; adapter = value.match(ADAPTER)) {
    if (seen.has(current)) throw new Error(`token --${name} has a var() cycle`);
    seen.add(current);
    current = adapter[1];
    value = readRaw(current, theme);
  }
  return value;
}
