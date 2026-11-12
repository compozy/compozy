import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";

import { cn } from "../../lib/utils";

export type TreeBranchNodeArm = "row" | "header";

export type TreeBranchProps = useRender.ComponentProps<"div">;

export interface TreeBranchNodeProps extends useRender.ComponentProps<"div"> {
  /**
   * Where the elbow lands on the node: `row` (default) is the center of a
   * 32 px row's first line (1rem from its top edge); `header` is the center
   * of a 28 px header row (0.875rem). Another geometry sets `--branch-arm`
   * through `className`.
   */
  arm?: TreeBranchNodeArm;
}

/** Branch rows indent past the parent's leading mark, whose center the rail continues. */
const TREE_BRANCH_CLASS = "relative pl-5";

/**
 * Each node's tree elbow ( |_ ): from the rail under the parent's mark into the
 * node's own mark. The rail is one straight line through every node but the
 * last, so a following sibling meets it without a seam (├); the last node
 * closes the branch with a rounded corner (└).
 *
 * `--branch-arm` is the arm's height from the node's top edge: the center of
 * the node's leading mark.
 */
const TREE_BRANCH_NODE_CLASS =
  "relative before:pointer-events-none before:absolute before:-top-0.5 before:-left-1.75 before:h-[calc(var(--branch-arm)+0.125rem)] before:w-3 before:border-b before:border-l before:border-line-strong last:before:rounded-bl-md after:pointer-events-none after:absolute after:-top-0.5 after:-bottom-0.5 after:-left-1.75 after:w-px after:bg-line-strong last:after:hidden";

const ARM_CLASS: Record<TreeBranchNodeArm, string> = {
  row: "[--branch-arm:1rem]",
  header: "[--branch-arm:0.875rem]",
};

/**
 * The rail a parent row's children hang from: the indent their elbows draw
 * into. Children are `TreeBranchNode`s; `render` swaps the element when the
 * branch is a list or a menu group.
 */
function TreeBranch({ className, render, ...props }: TreeBranchProps) {
  return useRender({
    defaultTagName: "div",
    render,
    props: mergeProps<"div">(
      { className: cn(TREE_BRANCH_CLASS, className) },
      { "data-slot": "tree-branch" } as Record<string, unknown>,
      props
    ),
    state: { slot: "tree-branch" },
  });
}

/**
 * One child on a `TreeBranch` rail, drawn with the branched tree connector the
 * session list uses for child sessions and subagents. The connector closes on
 * the last rendered node, so a filtered branch stays well-formed. `render`
 * draws it on the row element itself (a menu or command item) when a wrapper
 * would outlive a row that is filtered away.
 */
function TreeBranchNode({ arm = "row", className, render, ...props }: TreeBranchNodeProps) {
  return useRender({
    defaultTagName: "div",
    render,
    props: mergeProps<"div">(
      { className: cn(TREE_BRANCH_NODE_CLASS, ARM_CLASS[arm], className) },
      { "data-slot": "tree-branch-node", "data-arm": arm } as Record<string, unknown>,
      props
    ),
    state: { slot: "tree-branch-node", arm },
  });
}

export { TreeBranch, TreeBranchNode };
