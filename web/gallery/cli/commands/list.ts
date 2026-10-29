import { localCommentStore } from "../../node/local-comments.ts";
import { oneOf, stringFlag, type ParsedArgs } from "../args.ts";
import { discover } from "../registry.ts";

/** `proto list [--spec <id>] [--kind draft|component] [--json]`. */
export function list(args: ParsedArgs) {
  const spec = stringFlag(args, "spec");
  const kind = oneOf(stringFlag(args, "kind"), ["draft", "component"], "kind");
  const counts = localCommentStore().counts();
  const rows = discover()
    .entries.filter(entry => (!spec || entry.meta?.spec === spec) && (!kind || entry.kind === kind))
    .map(entry => ({
      slug: entry.slug,
      kind: entry.kind,
      title: entry.meta?.title ?? null,
      spec: entry.meta?.spec ?? null,
      surface: entry.meta?.surface ?? null,
      route: entry.meta?.route ?? null,
      states: entry.meta?.states.map(state => state.id) ?? [],
      file: entry.file,
      openComments: counts[entry.slug]?.open ?? 0,
    }));
  if (args.flags.json === true) {
    console.log(JSON.stringify(rows));
    return;
  }
  for (const row of rows) {
    console.log(
      [
        row.slug,
        row.kind,
        row.spec ?? "-",
        `${row.states.length} states`,
        `${row.openComments} open comments`,
        row.file,
      ].join("\t")
    );
  }
}
