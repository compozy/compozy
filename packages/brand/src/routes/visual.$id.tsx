import { createFileRoute, notFound } from "@tanstack/react-router";

import { Visual } from "../manual/art/visual";
import { parseVisualExportId } from "../manual/art/visuals";

// Export surface for one visual (_dx.md): /visual/<name>-<dark|light>, root [data-export].
// export:visuals screenshots that element; it carries no manual chrome.
export const Route = createFileRoute("/visual/$id")({
  loader: ({ params }) => {
    const parsed = parseVisualExportId(params.id);
    if (!parsed) throw notFound();
    return parsed;
  },
  head: ({ params }) => ({ meta: [{ title: `${params.id} · Compozy visual` }] }),
  component: VisualExport,
});

function VisualExport() {
  const { name, theme } = Route.useLoaderData();
  return (
    <div data-export={`${name}-${theme}`} className="relative h-[900px] w-[1600px] overflow-hidden">
      <Visual name={name} theme={theme} sizes="1600px" />
    </div>
  );
}
