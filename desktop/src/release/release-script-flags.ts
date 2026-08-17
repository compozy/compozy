import type { ReleaseArch, ReleasePlatform } from "./release-config";

export function parseReleaseScriptFlags(
  argv: readonly string[],
  allowed: readonly string[],
  required: readonly string[]
): ReadonlyMap<string, string> {
  if (argv.length % 2 !== 0) throw new Error("Release script flags require values.");
  const allowedNames = new Set(allowed);
  const values = new Map<string, string>();
  for (let index = 0; index < argv.length; index += 2) {
    const flag = argv[index];
    const value = argv[index + 1];
    if (!flag?.startsWith("--") || !value) throw new Error("Release script flags require values.");
    const name = flag.slice(2);
    if (!allowedNames.has(name)) throw new Error(`Unknown release script flag --${name}.`);
    if (values.has(name)) throw new Error(`Duplicate release script flag --${name}.`);
    values.set(name, value);
  }
  for (const name of required) {
    if (!values.get(name)?.trim()) throw new Error(`Release script flag --${name} is required.`);
  }
  return values;
}

export function parseReleasePlatform(value: string | undefined): ReleasePlatform {
  if (value !== "mac" && value !== "linux") throw new Error("Platform must be mac or linux.");
  return value;
}

export function parseReleaseArch(value: string | undefined): ReleaseArch {
  if (value !== "arm64" && value !== "x64") throw new Error("Architecture must be arm64 or x64.");
  return value;
}

export function requiredReleaseFlag(values: ReadonlyMap<string, string>, name: string): string {
  const value = values.get(name)?.trim();
  if (!value) throw new Error(`Release script flag --${name} is required.`);
  return value;
}

export function parseBooleanFlag(value: string | undefined, name: string): boolean {
  if (value === "true") return true;
  if (value === "false") return false;
  throw new Error(`Release script flag --${name} must be true or false.`);
}
