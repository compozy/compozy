/**
 * The gallery view lives in its URL, so a link reopens the same screen at every level: `/` (specs
 * index), `/?spec=sessions-kanban`, `/?p=session-board-card` (prototype, first state), or
 * `/?p=session-board-card&state=waiting&theme=light&w=390&compare=themes` (one state), or a
 * journey flow: `/?flow=board-to-pull-request&node=card&comments=1`. `spec`, `p` and `flow` are
 * exclusive: every gallery link goes through `selectionHref`/`select`, which drop the other
 * selections' parameters. `state` and `theme` keep the values of the standalone
 * prototype URL (`/<slug>?chrome=0&theme=…&state=…`) that the canvas iframe loads.
 */
import { prototypeParams, setParams } from "#gallery/contract";

export type Theme = "light" | "dark";

export type Viewport = "fit" | number;

export type Device = "desktop" | "tablet" | "phone";

/**
 * Viewport presets by device; the first width of each is the device's default. Any width from
 * `minWidth` to `maxWidth` is valid in `?w=` (the canvas edges drag to custom widths).
 */
export const devices: {
  id: Device;
  label: string;
  widths: readonly { width: number; label: string }[];
}[] = [
  {
    id: "desktop",
    label: "Desktop",
    widths: [
      { width: 1440, label: "Laptop" },
      { width: 1920, label: "Full HD display" },
      { width: 1280, label: "Small laptop" },
    ],
  },
  {
    id: "tablet",
    label: "Tablet",
    widths: [
      { width: 834, label: "iPad Air" },
      { width: 1024, label: "iPad Pro" },
      { width: 768, label: "iPad mini" },
    ],
  },
  {
    id: "phone",
    label: "Phone",
    widths: [
      { width: 390, label: "iPhone" },
      { width: 430, label: "iPhone Pro Max" },
      { width: 360, label: "Compact Android" },
    ],
  },
];

/** The device a width belongs to (phones below 600 px, tablets below 1180 px); none for "fit". */
export function deviceOf(width: Viewport): Device | null {
  if (width === "fit") return null;
  if (width < 600) return "phone";
  return width < 1180 ? "tablet" : "desktop";
}

export const minWidth = 320;
export const maxWidth = 3840;

/** The theme every surface starts in when the URL names none, like the app. */
export const defaultTheme: Theme = "dark";

export type GalleryView = {
  /** Spec overview (`?spec=`); with no selection at all the gallery shows the specs index. */
  spec: string | null;
  slug: string | null;
  state: string | null;
  theme: Theme | null;
  width: Viewport;
  compare: boolean;
  /** Comments panel open (`?comments=1`). */
  comments: boolean;
  /** Selected review comment (`?comment=<id>`). */
  comment: string | null;
  /** Flow on screen (`?flow=`) and its selected node (`?node=`). */
  flow: string | null;
  node: string | null;
};

function widthOf(value: string | null): Viewport {
  const width = Number(value);
  return Number.isInteger(width) && width >= minWidth && width <= maxWidth ? width : "fit";
}

export function themeOf(value: string | null): Theme | null {
  return value === "light" || value === "dark" ? value : null;
}

export function readView(search: string): GalleryView {
  const params = new URLSearchParams(search);
  return {
    spec: params.get("spec"),
    slug: params.get("p"),
    state: params.get(prototypeParams.state),
    theme: themeOf(params.get(prototypeParams.theme)),
    width: widthOf(params.get("w")),
    compare: params.get("compare") === "themes",
    comments: params.get("comments") === "1",
    comment: params.get("comment") || null,
    flow: params.get("flow"),
    node: params.get("node") || null,
  };
}

type ViewParams = Partial<
  Pick<GalleryView, "state" | "theme" | "width" | "compare" | "comments" | "comment" | "node">
>;

function paramsOf(view: ViewParams): Record<string, string | null> {
  const values: Record<string, string | null> = {};
  if ("state" in view) values[prototypeParams.state] = view.state ?? null;
  if ("theme" in view) {
    values[prototypeParams.theme] = view.theme && view.theme !== defaultTheme ? view.theme : null;
  }
  if ("width" in view) values.w = view.width === "fit" || !view.width ? null : String(view.width);
  if ("compare" in view) values.compare = view.compare ? "themes" : null;
  if ("comments" in view) values.comments = view.comments ? "1" : null;
  if ("comment" in view) values.comment = view.comment ?? null;
  if ("node" in view) values.node = view.node ?? null;
  return values;
}

/** Changes parameters inside the current selection (state, theme, width…); never adds history. */
export function updateView(patch: ViewParams) {
  setParams(paramsOf(patch));
}

/** What a gallery link opens, one per navigation level. */
export type Selection =
  | { kind: "index" }
  | { kind: "spec"; spec: string }
  | { kind: "prototype"; slug: string; state?: string | null }
  | { kind: "flow"; flow: string; node?: string | null };

/** Every gallery parameter except `theme`, which all selections share. */
const galleryParams = [
  "spec",
  "p",
  prototypeParams.state,
  "w",
  "compare",
  "comments",
  "comment",
  "flow",
  "node",
] as const;

/** Parameters each selection keeps; the others are cleared (the theme is always kept). */
const keptParams: Record<Selection["kind"], ReadonlySet<string>> = {
  index: new Set(),
  spec: new Set(["spec"]),
  prototype: new Set(["p", prototypeParams.state, "w", "compare", "comments", "comment"]),
  flow: new Set(["flow", "node", "comments", "comment"]),
};

function selectionParams(selection: Selection): Record<string, string | null> {
  const kept = keptParams[selection.kind];
  const values: Record<string, string | null> = {};
  for (const name of galleryParams) if (!kept.has(name)) values[name] = null;
  if (selection.kind === "spec") values.spec = selection.spec;
  if (selection.kind === "prototype") {
    values.p = selection.slug;
    values[prototypeParams.state] = selection.state ?? null;
    // The selected comment belongs to the prototype on screen; the panel stays open.
    values.comment = null;
  }
  if (selection.kind === "flow") {
    values.flow = selection.flow;
    values.node = selection.node ?? null;
    values.comment = null;
  }
  return values;
}

/**
 * Href for a selection: it sets its own parameters and clears every other selection's, keeping
 * `theme` (and, for prototypes, the viewport and theme comparison). Every gallery link uses it,
 * so a modified click (new tab) lands on the same URL as a plain click.
 */
export function selectionHref(selection: Selection, search: string) {
  const params = new URLSearchParams(search);
  for (const [name, value] of Object.entries(selectionParams(selection))) {
    if (value === null) params.delete(name);
    else params.set(name, value);
  }
  const query = params.toString();
  return query ? `/?${query}` : "/";
}

/** Plain click navigates in place; modified clicks keep the link's default (new tab). */
export function plainClick(event: {
  metaKey: boolean;
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  button: number;
  preventDefault: () => void;
}) {
  if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
    return false;
  }
  event.preventDefault();
  return true;
}

/** Same rules as `selectionHref`, adding a history entry (Back returns to the previous view). */
export function select(selection: Selection) {
  setParams(selectionParams(selection), { push: true });
}

/** Absolute link to a selection, for the "copy link" menu. */
export function selectionUrl(selection: Selection, search: string, base: string) {
  return new URL(selectionHref(selection, search), base).href;
}

type Catalog = {
  prototypeExists: (slug: string) => boolean;
  specExists: (spec: string) => boolean;
  /** Node ids of a flow, or null when no such flow exists. */
  flowNodes?: (flow: string) => readonly string[] | null;
};

export type ResolvedSelection =
  | { kind: "flow"; flow: string; node: string | null }
  | { kind: "missing-flow"; flow: string }
  | { kind: "prototype"; slug: string }
  | { kind: "spec"; spec: string }
  | { kind: "index" };

/**
 * What the URL selects: an existing `flow` wins over `p`, which wins over `spec`. A missing flow
 * shows "Flow not found" unless `p` names an existing prototype; an unknown `p` or `spec` falls
 * back to the next level up (the spec, then the index). A stale `node` selects no node.
 */
export function resolveSelection(view: GalleryView, catalog: Catalog): ResolvedSelection {
  const prototype = view.slug && catalog.prototypeExists(view.slug) ? view.slug : null;
  if (view.flow) {
    const nodes = catalog.flowNodes?.(view.flow) ?? null;
    if (nodes) {
      const node = view.node && nodes.includes(view.node) ? view.node : null;
      return { kind: "flow", flow: view.flow, node };
    }
    if (!prototype) return { kind: "missing-flow", flow: view.flow };
  }
  if (prototype) return { kind: "prototype", slug: prototype };
  if (view.spec && catalog.specExists(view.spec)) return { kind: "spec", spec: view.spec };
  return { kind: "index" };
}

/**
 * Parameters to rewrite (without history) so the URL names only what is on screen. Reads the raw
 * query: a present parameter is cleared whether or not its value is valid or empty, and a theme
 * equal to the default is dropped.
 */
export function canonicalParams(
  search: string,
  resolved: ResolvedSelection,
  state: string | null
): Record<string, string | null> | null {
  // A missing flow keeps its URL, so the notice can name it.
  if (resolved.kind === "missing-flow") return null;
  const params = new URLSearchParams(search);
  const kept = keptParams[resolved.kind];
  const patch: Record<string, string | null> = {};
  for (const name of galleryParams) {
    if (params.has(name) && !kept.has(name)) patch[name] = null;
  }
  const rawState = params.get(prototypeParams.state);
  if (resolved.kind === "prototype" && rawState !== null && rawState !== state) {
    patch[prototypeParams.state] = state;
  }
  const rawTheme = params.get(prototypeParams.theme);
  if (rawTheme !== null && (themeOf(rawTheme) ?? defaultTheme) === defaultTheme) {
    patch[prototypeParams.theme] = null;
  }
  const rawWidth = params.get("w");
  if (resolved.kind === "prototype" && rawWidth !== null && widthOf(rawWidth) === "fit") {
    patch.w = null;
  }
  const rawComments = params.get("comments");
  if (resolved.kind === "prototype" && rawComments !== null && rawComments !== "1") {
    patch.comments = null;
  }
  const panel = resolved.kind === "prototype" || resolved.kind === "flow";
  if (resolved.kind === "flow" && params.has("node") && params.get("node") !== resolved.node) {
    patch.node = resolved.node;
  }
  if (resolved.kind === "flow" && rawComments !== null && rawComments !== "1") {
    patch.comments = null;
  }
  if (panel && params.get("comment") === "") patch.comment = null;
  return Object.keys(patch).length > 0 ? patch : null;
}

/**
 * The standalone prototype URL the canvas loads (also the `proto screenshot` contract); with
 * `toolbar`, the floating prototype toolbar stays (for "open in a new tab").
 */
export function frameSrc(
  slug: string,
  view: { state: string; theme: Theme | null },
  toolbar = false
) {
  const params = new URLSearchParams(toolbar ? {} : { [prototypeParams.chrome]: "0" });
  if (view.theme) params.set(prototypeParams.theme, view.theme);
  if (view.state) params.set(prototypeParams.state, view.state);
  const query = params.toString();
  return `/${slug}${query ? `?${query}` : ""}`;
}
