import { createFileRoute } from "@tanstack/react-router";

import { kitIndex } from "../manual/templates/registry";

// /kit/index.json (_dx.md): `[{ id, w, h, group }]` for every kit template. Served by the dev
// server and prerendered to .output/public/kit/index.json; export:kit reads it.
export const Route = createFileRoute("/kit/index.json")({
  server: {
    handlers: {
      GET: () => Response.json(kitIndex()),
    },
  },
});
