import { createFileRoute } from "@tanstack/react-router";

import { NotFound } from "../manual/parts/not-found";

// Static not-found artifact: prerendered to .output/public/404.html for static hosts (IT-002).
export const Route = createFileRoute("/404")({
  head: () => ({ meta: [{ title: "Not found · Compozy brand" }] }),
  component: NotFound,
});
