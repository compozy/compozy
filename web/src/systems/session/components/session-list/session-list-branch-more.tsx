import { Button, TreeBranchNode } from "@compozy/ui";


export interface SessionListBranchMoreProps {
  /** Rows the next press reveals; 0 once every row shows. */
  count: number;
  onShowMore: () => void;
  /** Folds the list back to its short default; absent where a list only grows. */
  onShowLess?: () => void;
  /** A press is already reading the rows it reveals. */
  disabled?: boolean;
  testId: string;
}

/**
 * The last node of a bounded branch list: "Show {N} more" while rows sit past
 * the shown ones, then "Show less" where the list can fold back. Child
 * sessions and subagents share it, so both lists end the same way.
 */
export function SessionListBranchMore({
  count,
  onShowMore,
  onShowLess,
  disabled = false,
  testId,
}: SessionListBranchMoreProps) {
  if (count <= 0 && !onShowLess) return null;
  return (
    <TreeBranchNode className="px-0.5 py-0.5">
      <Button
        type="button"
        variant="ghost"
        size="xs"
        disabled={disabled}
        onClick={count > 0 ? onShowMore : onShowLess}
        data-testid={testId}
      >
        {count > 0 ? `Show ${count} more` : "Show less"}
      </Button>
    </TreeBranchNode>
  );
}
