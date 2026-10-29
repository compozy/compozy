import { existsSync, mkdirSync, realpathSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, relative, resolve, sep } from "node:path";

import { maxSlugLength, reservedSlugPrefix, slugPattern } from "../../node/discovery.ts";
import { repoRoot, webRoot } from "../../node/paths.ts";
import { requirePositional, stringFlag, UsageError, type ParsedArgs } from "../args.ts";
import { discover } from "../registry.ts";

function template(meta: Record<string, unknown>, draft: boolean) {
  const body = draft
    ? `export default function Prototype({ state }: PrototypeProps) {\n  return <main data-state={state} />;\n}\n`
    : `export default function Prototype({ state }: PrototypeProps) {\n  void state;\n  return null;\n}\n`;
  return `import { definePrototype, type PrototypeProps } from "#gallery/contract";\n\nexport const meta = definePrototype(${JSON.stringify(meta, null, 2)});\n\n${body}`;
}

/** The folder must stay inside web/ after resolving symlinks of its closest existing ancestor. */
/** The folder as `web/`-relative POSIX path with symlinks resolved (its missing tail kept). */
function canonicalLocal(folder: string) {
  let ancestor = folder;
  while (!existsSync(ancestor)) ancestor = dirname(ancestor);
  const canonical = resolve(realpathSync(ancestor), relative(ancestor, folder));
  const local = relative(realpathSync(webRoot), canonical).split(sep).join("/");
  if (local.startsWith("..") || isAbsolute(local)) {
    throw new UsageError("Prototype path escapes web/");
  }
  return local;
}

/** `proto new <slug> --in <dir> | --draft [--spec <id>] [--title <text>]`. */
export function create(args: ParsedArgs) {
  const slug = requirePositional(args, "slug");
  const into = stringFlag(args, "in");
  const draft = args.flags.draft === true;
  const spec = stringFlag(args, "spec");
  if (
    !slugPattern.test(slug) ||
    slug.length > maxSlugLength ||
    slug.startsWith(reservedSlugPrefix)
  ) {
    throw new UsageError(`Invalid prototype slug "${slug}"`);
  }
  if (Boolean(into) === draft) throw new UsageError("Use exactly one of --in <dir> or --draft");
  const { catalog, entries } = discover();
  if (spec && catalog.available && !catalog.specs.some(item => item.id === spec)) {
    throw new UsageError(`Unknown spec "${spec}" (no .compozy/tasks/${spec}/)`);
  }
  const existing = entries.find(entry => entry.slug === slug);
  if (existing) throw new UsageError(`Prototype slug "${slug}" already exists: ${existing.file}`);
  const folder = draft ? resolve(webRoot, "src/drafts", slug) : resolve(repoRoot, into ?? "");
  // Checked on the resolved path, so a symlink cannot point --in outside the allowed roots.
  const local = canonicalLocal(folder);
  if (!draft && !/^src\/(systems|components)(\/|$)/.test(local)) {
    throw new UsageError("--in must be inside web/src/systems or web/src/components");
  }
  const file = resolve(folder, draft ? "index.proto.tsx" : `${slug}.proto.tsx`);
  const meta = {
    title: stringFlag(args, "title") ?? slug,
    ...(spec ? { spec } : {}),
    states: [{ id: "default", label: "Default" }],
  };
  mkdirSync(folder, { recursive: true });
  writeFileSync(file, template(meta, draft), { flag: "wx" });
  console.log(`Created ${relative(repoRoot, file)}`);
}
