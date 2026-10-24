interface MdastNode {
  type: string;
  value?: string;
  children?: MdastNode[];
}

function splitLineBreaks(value: string): MdastNode[] {
  const nodes: MdastNode[] = [];
  value.split("\n").forEach((line, index) => {
    if (index > 0) nodes.push({ type: "break" });
    if (line) nodes.push({ type: "text", value: line });
  });
  return nodes;
}

function visitLineBreaks(node: MdastNode) {
  if (!node.children) return;
  node.children = node.children.flatMap(child => {
    if (child.type === "text" && child.value?.includes("\n")) return splitLineBreaks(child.value);
    visitLineBreaks(child);
    return [child];
  });
}

/**
 * Remark plugin for typed chat input: every line break the author typed inside
 * a paragraph renders as a line break, not the space Markdown folds it into.
 * Code keeps its own text; block structure (lists, quotes) is unchanged.
 */
export function remarkAuthoredLineBreaks() {
  return (tree: MdastNode) => visitLineBreaks(tree);
}
