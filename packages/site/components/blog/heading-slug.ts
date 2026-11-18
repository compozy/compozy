/**
 * The anchor of a post heading. The heading renders it as its `id` and the section rail links to
 * it, so both read the same text through this one function.
 */
export function headingSlug(heading: string): string {
  return heading
    .replace(/<[^>]+>/g, "")
    .replace(/`([^`]+)`/g, "$1")
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s-]/gu, "")
    .replace(/\s+/g, "-");
}

/**
 * The post's numbered sections: its second-level headings, as the rail lists them. Each anchor is
 * the heading's own id, derived from the same text.
 */
export function railSections(toc: ReadonlyArray<{ title: string }>) {
  return toc.map(item => ({ id: headingSlug(item.title), text: item.title }));
}
