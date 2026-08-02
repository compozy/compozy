import { createFileRoute, notFound } from "@tanstack/react-router";

export const Route = createFileRoute("/_app/marketplace/$kind_")({
  beforeLoad: () => {
    throw notFound();
  },
});
