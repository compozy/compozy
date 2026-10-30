/** How flow nodes are named and opened, shared by the node cards, the canvas and the inspector. */
import { externalLabels, type ResolvedNode } from "#gallery/flow";
import { select, type Selection } from "#gallery/shell/view";

type PrototypeStep = Extract<ResolvedNode, { kind: "prototype" }>;
type ExternalStep = Extract<ResolvedNode, { kind: "external" }>;

/** `https://github.com/compozy/compozy/pull/1` → `github.com/compozy/compozy/pull/1`. */
export function displayUrl(url: string) {
  return url.replace(/^https:\/\//, "");
}

export function externalName(node: ExternalStep) {
  return node.label || externalLabels[node.external];
}

export function nodeName(node: ResolvedNode) {
  return node.kind === "prototype" ? node.title : externalName(node);
}

/** What a step is called in comments and links: `3. Session list row · Running`. */
export function stepLabel(node: ResolvedNode) {
  return `${node.step}. ${nodeName(node)}${node.kind === "prototype" ? ` · ${node.stateLabel}` : ""}`;
}

/** The gallery selection that opens a prototype node at its state. */
export function prototypeSelection(node: PrototypeStep): Selection {
  return { kind: "prototype", slug: node.proto, state: node.state };
}

/** Opens the node: a prototype at its state in the gallery, an external URL in a new tab. */
export function openNode(node: ResolvedNode) {
  if (node.kind === "prototype") select(prototypeSelection(node));
  else if (node.url) window.open(node.url, "_blank", "noopener,noreferrer");
}
