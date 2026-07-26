import { Button, Eyebrow, Progress, cn } from "@agh/ui";

import {
  evenWeights,
  layoutNodeIds,
  createLayoutNodeIdFactory,
  layoutOrientationOf,
  toSplit,
  toStack,
  type LayoutOrientation,
} from "../../lib/window-manager-layout-mutations";
import { fractionLabel } from "../../lib/window-manager-layout-detents";
import { layoutNodeWindowIds, replaceLayoutNode } from "../../lib/window-manager-layout-tree";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutNode,
} from "../../lib/window-manager-layout-types";
import {
  LayoutColumnsDiagram,
  LayoutRowsDiagram,
  LayoutStackDiagram,
} from "./layout-node-diagrams";

interface LayoutInspectorNodeProps {
  node: WindowManagerLayoutNode;
  desktop: WindowManagerLayoutDesktop;
  onDesktopChange: (next: WindowManagerLayoutDesktop) => void;
}

/**
 * How this branch arranges the windows under it. Three pictures, not an axis
 * enum and a weight column — and the "Arrange as" actions are the only writer of
 * split weights besides the seam drag, so both go through the same normalizer.
 */
export function LayoutInspectorNode({ node, desktop, onDesktopChange }: LayoutInspectorNodeProps) {
  const windowIds = layoutNodeWindowIds(node);
  const canRestructure = windowIds.length >= 2;
  const orientation = node.kind === "split" ? layoutOrientationOf(node.axis) : null;

  const mintId = () =>
    createLayoutNodeIdFactory(desktop.groups.flatMap(group => layoutNodeIds(group.root)));

  const replace = (next: WindowManagerLayoutNode) => {
    onDesktopChange(replaceLayoutNode(desktop, node.id, next));
  };

  return (
    <>
      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Arrange as</Eyebrow>
        <div className="flex flex-wrap gap-1.5">
          <ArrangeAction
            active={orientation === "rows"}
            disabled={!canRestructure}
            label="Rows"
            onSelect={() => replace(toSplit(node, "rows", mintId()))}
          >
            <LayoutRowsDiagram className="h-4 w-6.5" />
          </ArrangeAction>
          <ArrangeAction
            active={orientation === "columns"}
            disabled={!canRestructure}
            label="Columns"
            onSelect={() => replace(toSplit(node, "columns", mintId()))}
          >
            <LayoutColumnsDiagram className="h-4 w-6.5" />
          </ArrangeAction>
          <ArrangeAction
            active={node.kind === "stack"}
            disabled={!canRestructure}
            label="Stack"
            onSelect={() => replace(toStack(node, mintId()))}
          >
            <LayoutStackDiagram className="h-4 w-6.5" />
          </ArrangeAction>
        </div>
        {canRestructure ? null : (
          <p className="text-form-hint text-subtle">
            A single window has nothing to split. Add a second window to this branch first.
          </p>
        )}
      </section>

      {node.kind === "split" ? (
        <LayoutSplitBalance
          node={node}
          orientation={orientation ?? "columns"}
          onDistribute={() => replace({ ...node, weights: evenWeights(node.children.length) })}
        />
      ) : null}
    </>
  );
}

function ArrangeAction({
  active,
  disabled,
  label,
  children,
  onSelect,
}: {
  active: boolean;
  disabled: boolean;
  label: string;
  children: React.ReactNode;
  onSelect: () => void;
}) {
  return (
    <button
      aria-pressed={active}
      className={cn(
        "flex w-16 flex-col items-center gap-1.5 rounded-md border border-line px-1 py-2",
        "bg-btn-default-fill text-form-hint font-medium text-muted",
        "transition-colors duration-base ease-out",
        "hover:not-disabled:border-line-strong hover:not-disabled:bg-btn-default-hover hover:not-disabled:text-fg-strong",
        "focus-visible:outline-none focus-visible:shadow-focus-ring",
        "disabled:cursor-not-allowed disabled:opacity-35",
        active && "border-accent-dim bg-accent-tint text-accent-strong"
      )}
      data-testid={`layout-inspector-arrange-${label.toLowerCase()}`}
      disabled={disabled}
      type="button"
      onClick={onSelect}
    >
      {children}
      <span>{label}</span>
    </button>
  );
}

function LayoutSplitBalance({
  node,
  orientation,
  onDistribute,
}: {
  node: Extract<WindowManagerLayoutNode, { kind: "split" }>;
  orientation: LayoutOrientation;
  onDistribute: () => void;
}) {
  const unit = orientation === "rows" ? "Row" : "Column";

  return (
    <section className="flex flex-col gap-2">
      <Eyebrow className="text-faint">Balance</Eyebrow>
      <div className="flex flex-col gap-1.5">
        {node.children.map((child, index) => (
          <Progress
            className="items-center gap-2"
            key={child.id}
            max={1}
            value={node.weights[index] ?? 0}
          >
            <span className="w-16 shrink-0 text-form-hint text-subtle">
              {unit} {index + 1}
            </span>
            <span className="order-last w-9 shrink-0 text-right font-mono text-form-hint tabular-nums text-fg">
              {fractionLabel(node.weights[index] ?? 0)}
            </span>
          </Progress>
        ))}
      </div>
      <Button
        className="self-start"
        size="xs"
        type="button"
        variant="outline"
        onClick={onDistribute}
      >
        Distribute evenly
      </Button>
    </section>
  );
}
