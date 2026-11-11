/** Page bodies come straight from the site's MDX, loaded on demand: nothing is copied into the draft. */
import { parseDoc, type Doc } from "./mdx";

const ROOT = "../../../../packages/site/content/docs/";
const FILES = import.meta.glob<string>("../../../../packages/site/content/docs/**/*.mdx", {
  query: "?raw",
  import: "default",
});

export async function loadDoc(slug: string): Promise<Doc | null> {
  const load = FILES[`${ROOT}${slug || "index"}.mdx`] ?? FILES[`${ROOT}${slug}/index.mdx`];
  if (!load) return null;
  return parseDoc(await load());
}
