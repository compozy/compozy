import { createFileRoute, notFound } from "@tanstack/react-router";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";

import { fieldsOf, findTemplate } from "../manual/templates/registry";

// Export surface for one kit template (_dx.md): /kit/<id> renders it at native pixel size in the
// root [data-kit], which gains data-kit-ready once fonts and images have settled. export:kit
// screenshots that element. `?field=value` overrides any text field before the capture.
export const Route = createFileRoute("/kit/$id")({
  loader: ({ params }) => {
    if (!findTemplate(params.id)) throw notFound();
    return { id: params.id };
  },
  head: ({ params }) => ({ meta: [{ title: `kit/${params.id} · Compozy` }] }),
  component: KitExport,
});

const noSubscription = () => () => {};

/** Fonts loaded and every image in `root` decoded (a broken image settles too; the PNG shows it). */
async function settle(root: HTMLElement) {
  await document.fonts.ready;
  await Promise.all(
    Array.from(root.querySelectorAll("img"), async img => {
      if (!img.complete) {
        await new Promise(done => {
          img.addEventListener("load", done, { once: true });
          img.addEventListener("error", done, { once: true });
        });
      }
      await img.decode().catch(() => undefined);
    })
  );
}

function KitExport() {
  const { id } = Route.useLoaderData();
  const found = findTemplate(id);
  if (!found) throw notFound();
  const { spec, template } = found;
  const Template = template.component;
  // Prerendered HTML carries the defaults; the client applies ?field=value after hydration.
  const search = useSyncExternalStore(
    noSubscription,
    () => window.location.search,
    () => ""
  );
  const overrides = Object.fromEntries(new URLSearchParams(search));
  const root = useRef<HTMLDivElement>(null);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const el = root.current;
    if (!el || search !== window.location.search) return;
    let live = true;
    void settle(el).then(() => {
      if (live) setReady(true);
    });
    return () => {
      live = false;
    };
  }, [search]);

  return (
    <div
      ref={root}
      data-kit={spec.id}
      data-kit-ready={ready ? "" : undefined}
      className="relative overflow-hidden [container-type:inline-size]"
      style={{ width: spec.w, height: spec.h }}
    >
      <Template {...fieldsOf(template, overrides)} />
    </div>
  );
}
