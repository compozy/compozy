import { prototypeParams } from "../../contract.ts";
import { galleryOrigin } from "../../node/paths.ts";
import { oneOf, requirePositional, stringFlag, UsageError, type ParsedArgs } from "../args.ts";
import { findEntry } from "../registry.ts";

/** The standalone URL (`/<slug>?chrome=0…`), or the gallery view with `gallery`. */
export function prototypeUrl(
  slug: string,
  options: { state?: string; theme?: string; gallery?: boolean },
  origin = galleryOrigin
) {
  const url = new URL(options.gallery ? "/" : `/${slug}`, origin);
  if (options.gallery) url.searchParams.set("p", slug);
  else url.searchParams.set(prototypeParams.chrome, "0");
  if (options.state) url.searchParams.set(prototypeParams.state, options.state);
  if (options.theme) url.searchParams.set(prototypeParams.theme, options.theme);
  return url;
}

/** Validates the slug and state against discovery. */
export function resolveTarget(args: ParsedArgs) {
  const slug = requirePositional(args, "slug");
  const entry = findEntry(slug);
  const state = stringFlag(args, "state");
  if (state && !entry.meta?.states.some(option => option.id === state)) {
    throw new UsageError(
      `Unknown state "${state}" for ${slug}: ${entry.meta?.states.map(option => option.id).join(", ")}`
    );
  }
  const theme = oneOf(stringFlag(args, "theme"), ["light", "dark"], "theme");
  return { slug, entry, state, theme };
}

/** `proto url <slug> [--state <id>] [--theme light|dark] [--gallery]`. */
export function url(args: ParsedArgs) {
  const { slug, state, theme } = resolveTarget(args);
  console.log(prototypeUrl(slug, { state, theme, gallery: args.flags.gallery === true }).href);
}
