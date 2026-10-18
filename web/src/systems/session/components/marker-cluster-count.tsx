import { MarkerMeta } from "@compozy/ui";

/** Faint ×N tabular count appended when consecutive same-kind events clustered. */
export function ClusterCount({ count }: { count: number }) {
  if (count <= 1) return null;
  return <MarkerMeta data-testid="marker-cluster-count"> ×{count}</MarkerMeta>;
}
