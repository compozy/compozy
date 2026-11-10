/** The draft's whole state: which page is open and which overlay, if any. */
export interface DocsView {
  slug: string;
  /** The search query while the palette is open; `null` when it is closed. */
  query: string | null;
  menu: boolean;
  /** The language the SDK reference is read in. */
  lang: "ts" | "go";
}

export function withSearch(view: DocsView): DocsView {
  return { ...view, query: view.query ?? "", menu: false };
}

export function withoutOverlays(view: DocsView): DocsView {
  return view.query === null && !view.menu ? view : { ...view, query: null, menu: false };
}
