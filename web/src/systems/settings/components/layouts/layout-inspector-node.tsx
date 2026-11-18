import { Button, Eyebrow, Progress, cn } from "@compozy/ui";

import { fractionLabel } from "../../lib/window-manager-layout-detents";
import {
  evenWeights,
  setLayoutSplitAxis,
  setLayoutSplitWeights,
} from "../../lib/window-manager-layout-tree";
import type {
  WindowManagerLayoutAxis,
  WindowManagerLayoutNode,
  WindowManagerLayoutSplit,
} from "../../lib/window-manager-layout-types";
import { LayoutColumnsDiagram, LayoutRowsDiagram } from "./layout-node-diagrams";

/**
 * What the person chooses. The daemon's `horizontal` axis lays members out side
 * by side, so "columns" is `horizontal` and "rows" is `vertical` — the inverse
 * of what the words suggest. The mapping lives here, once; the UI never says
 * "axis".
 */
const ORIENTATIONS: ReadonlyArray<{
  axis: WindowManagerLayoutAxis;
  label: "Rows" | "Columns";
  unit: string;
  Diagram: typeof LayoutRowsDiagram;
}> = [
  { axis: "vertical", label: "Rows", unit: "Row", Diagram: LayoutRowsDiagram },
  { axis: "horizontal", label: "Columns", unit: "Column", Diagram: LayoutColumnsDiagram },
];

interface LayoutInspectorSplitProps {
  root: WindowManagerLayoutNode;
  split: WindowManagerLayoutSplit;
  onRootChange: (next: WindowManagerLayoutNode) => void;
}

/**
 * The split the selected window sits in: which way it runs and how its members
 * share the space. Changing the direction to match an enclosing split merges
 * the two, as the daemon requires; seams on the canvas and "Distribute evenly"
 * go through the same weight normalizer.
 */
export function LayoutInspectorSplit({ root, split, onRootChange }: LayoutInspectorSplitProps) {
  const current = ORIENTATIONS.find(orientation => orientation.axis === split.axis);

  return (
    <>
      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Split as</Eyebrow>
        <div className="flex flex-wrap gap-1.5">
          {ORIENTATIONS.map(orientation => (
            <button
              aria-pressed={orientation.axis === split.axis}
              className={cn(
                "flex w-16 flex-col items-center gap-1.5 rounded-lg px-1 py-2",
                "text-form-hint font-medium text-muted-foreground",
                "transition-colors duration-base ease-out",
                "focus-visible:outline-none focus-visible:shadow-focus-ring",
                orientation.axis === split.axis
                  ? "bg-selected text-fg"
                  : "bg-card shadow-card hover:bg-surface-2 hover:text-fg"
              )}
              data-testid={`layout-inspector-arrange-${orientation.label.toLowerCase()}`}
              key={orientation.axis}
              type="button"
              onClick={() => onRootChange(setLayoutSplitAxis(root, split.id, orientation.axis))}
            >
              <orientation.Diagram className="h-4 w-6.5" />
              <span>{orientation.label}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Balance</Eyebrow>
        <div className="flex flex-col gap-1.5">
          {split.children.map((child, index) => (
            <Progress
              className="items-center gap-2"
              key={child.id}
              max={1}
              value={split.weights[index] ?? 0}
            >
              <span className="w-16 shrink-0 text-form-hint text-subtle">
                {current?.unit ?? "Member"} {index + 1}
              </span>
              <span className="order-last w-9 shrink-0 text-right font-mono text-form-hint tabular-nums text-fg">
                {fractionLabel(split.weights[index] ?? 0)}
              </span>
            </Progress>
          ))}
        </div>
        <Button
          className="self-start"
          data-testid="layout-inspector-distribute"
          size="xs"
          type="button"
          variant="secondary"
          onClick={() =>
            onRootChange(setLayoutSplitWeights(root, split.id, evenWeights(split.children.length)))
          }
        >
          Distribute evenly
        </Button>
      </section>
    </>
  );
}
