import { createFileRoute } from "@tanstack/react-router";

import { DesignSystemShowcase } from "@/components/design-system-showcase";
import { useRouterScrollTracking } from "@/hooks/use-router-scroll-tracking";

export const Route = createFileRoute("/design-system")({
  component: DesignSystemPage,
});

function DesignSystemPage() {
  // The page outside the shell restores scroll by the router (`restoreScrollOutsideShell`).
  useRouterScrollTracking();
  return <DesignSystemShowcase />;
}
