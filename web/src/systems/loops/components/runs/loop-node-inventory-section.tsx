import type { ReactNode } from "react";
import { Layers } from "lucide-react";

import { Section } from "@compozy/ui";

/** The inventory's section, shared by the live view and its skeleton so the page's frame matches. */
export function LoopNodeInventorySection({ children }: { children: ReactNode }) {
  return (
    <Section
      data-testid="loop-node-inventory"
      icon={Layers}
      label="Inventory"
      bodyClassName="gap-4"
    >
      {children}
    </Section>
  );
}
