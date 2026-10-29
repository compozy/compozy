/**
 * The prototype contract. A prototype is a `*.proto.tsx` file next to the view it renders:
 *
 *   export const meta = definePrototype({ title, spec?, surface?, route?, states: [...] });
 *   export default function Prototype({ state }: PrototypeProps) { ... }
 *
 * `meta` must be a pure literal: discovery reads it from the syntax tree and never runs the
 * module. `spec` names a `.compozy/tasks/<id>/` directory. The URL helpers below keep the
 * standalone prototype URL (`/<slug>?state=&theme=light|dark&chrome=0`) as the only state.
 */
export type PrototypeState = { id: string; label: string };

export type PrototypeMeta = {
  title: string;
  /** Spec id, the name of a `.compozy/tasks/<id>/` directory. */
  spec?: string;
  /** Short surface code from the spec's UI contract (`S5`), shown beside the title. */
  surface?: string;
  /** Production route the view belongs to, for reviewers; never navigated. */
  route?: string;
  states: readonly [PrototypeState, ...PrototypeState[]];
};

export type PrototypeProps = { state: string };

export function definePrototype<const Meta extends PrototypeMeta>(meta: Meta): Meta {
  return meta;
}

/** Search parameters the standalone prototype URL reads. */
export const prototypeParams = { state: "state", theme: "theme", chrome: "chrome" } as const;

const listeners = new Set<() => void>();

/** Subscribes to URL changes made by `setParams` and by history navigation. */
export function subscribeSearch(listener: () => void) {
  listeners.add(listener);
  window.addEventListener("popstate", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("popstate", listener);
  };
}

/** Updates several query parameters at once; `push` adds a history entry (Back returns). */
export function setParams(values: Record<string, string | null>, options?: { push?: boolean }) {
  const url = new URL(window.location.href);
  for (const [name, value] of Object.entries(values)) {
    if (value === null) url.searchParams.delete(name);
    else url.searchParams.set(name, value);
  }
  if (url.href === window.location.href) return;
  if (options?.push) window.history.pushState(null, "", url);
  else window.history.replaceState(null, "", url);
  for (const listener of listeners) listener();
}

/** Link to another prototype, keeping the theme and chrome parameters of the current one. */
export function protoHref(slug: string, state?: string) {
  const current = new URLSearchParams(window.location.search);
  const next = new URLSearchParams();
  for (const key of [prototypeParams.theme, prototypeParams.chrome]) {
    const value = current.get(key);
    if (value) next.set(key, value);
  }
  if (state) next.set(prototypeParams.state, state);
  const query = next.toString();
  return `/${slug}${query ? `?${query}` : ""}`;
}
