/** Branch rows indent past the parent's status mark, whose center the rail continues. */
export const BRANCH_RAIL = "relative pl-5";

/**
 * Each branch row's tree elbow ( |_ ): from the rail under the parent's mark
 * into the row's own mark on its title line. The rail continues past a row
 * only while a sibling follows, so the last row closes the branch.
 */
export const BRANCH_NODE =
  "relative before:pointer-events-none before:absolute before:-top-0.5 before:-left-1.75 before:h-4.5 before:w-3 before:rounded-bl-md before:border-b before:border-l before:border-line-strong after:pointer-events-none after:absolute after:top-3.5 after:-bottom-0.5 after:-left-1.75 after:w-px after:bg-line-strong last:after:hidden";
