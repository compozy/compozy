import type { TopbarSlotValue } from "@compozy/ui";

/** What the window head shows for a Loop's detail: back to the catalog, the Loop as the leaf. */
export function loopDetailTopbarIdentity(
  name: string,
  backToLoops: () => void
): Pick<TopbarSlotValue, "crumb" | "crumbs" | "onBack"> {
  return {
    onBack: backToLoops,
    crumbs: [{ id: "loops", label: "Loops", onSelect: backToLoops }],
    crumb: name,
  };
}
