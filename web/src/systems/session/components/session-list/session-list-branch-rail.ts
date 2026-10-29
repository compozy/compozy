/** Branch rows indent past the parent's status mark, whose center the rail continues. */
export const BRANCH_RAIL = "relative pl-5";

/**
 * Each branch row's tree elbow ( |_ ): from the rail under the parent's mark
 * into the row's own mark. The rail is one straight line through every row
 * but the last, so a following sibling meets it without a seam (├); the last
 * row closes the branch with a rounded corner (└).
 *
 * `--branch-arm` is the arm's height from the row's top edge: the center of
 * the row's mark. It defaults to a session row's title line (6 px padding +
 * half its 20 px line); a row with another geometry sets its own.
 */
export const BRANCH_NODE =
  "relative [--branch-arm:1rem] before:pointer-events-none before:absolute before:-top-0.5 before:-left-1.75 before:h-[calc(var(--branch-arm)+0.125rem)] before:w-3 before:border-b before:border-l before:border-line-strong last:before:rounded-bl-md after:pointer-events-none after:absolute after:-top-0.5 after:-bottom-0.5 after:-left-1.75 after:w-px after:bg-line-strong last:after:hidden";

/** The arm height of a 28 px header row (a project inside a project folder). */
export const BRANCH_NODE_HEADER_ARM = "[--branch-arm:0.875rem]";
