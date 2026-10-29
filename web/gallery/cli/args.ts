/**
 * A small argument parser for `bun run proto`: positionals plus `--flag`, `--flag value` and
 * `--flag=value`. Each command declares its flags, so a typo fails instead of being ignored.
 */
export type FlagSpec = Record<string, "boolean" | "string">;

export type ParsedArgs = {
  positionals: string[];
  flags: Record<string, string | boolean | undefined>;
};

export class UsageError extends Error {}

export function parseArgs(argv: readonly string[], spec: FlagSpec): ParsedArgs {
  const positionals: string[] = [];
  const flags: ParsedArgs["flags"] = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index] ?? "";
    if (!arg.startsWith("--")) {
      positionals.push(arg);
      continue;
    }
    const [name = "", inline] = arg.slice(2).split(/=(.*)/s, 2);
    const kind = spec[name];
    if (!kind) throw new UsageError(`Unknown flag --${name}`);
    if (kind === "boolean") {
      if (inline !== undefined) throw new UsageError(`--${name} takes no value`);
      flags[name] = true;
      continue;
    }
    const value = inline ?? argv[index + 1];
    if (value === undefined || (inline === undefined && value.startsWith("--"))) {
      throw new UsageError(`--${name} needs a value`);
    }
    if (inline === undefined) index += 1;
    flags[name] = value;
  }
  return { positionals, flags };
}

export function stringFlag(args: ParsedArgs, name: string) {
  const value = args.flags[name];
  return typeof value === "string" ? value : undefined;
}

export function oneOf<const T extends string>(
  value: string | undefined,
  options: readonly T[],
  name: string
): T | undefined {
  if (value === undefined) return undefined;
  if ((options as readonly string[]).includes(value)) return value as T;
  throw new UsageError(`--${name} must be one of ${options.join(", ")}`);
}

export function requirePositional(args: ParsedArgs, label: string) {
  const value = args.positionals[0];
  if (!value) throw new UsageError(`Missing <${label}>`);
  return value;
}
