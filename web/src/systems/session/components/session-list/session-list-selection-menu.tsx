import { ContextMenuGroup, ContextMenuLabel, MenuTreeItems, type MenuTreeHost } from "@compozy/ui";

import type { SessionSelectionVerbs } from "../../lib/session-selection-verbs";
import { sessionCountLabel } from "../../lib/session-folders";
import { selectionMenuTree } from "../../lib/session-selection-menu-tree";

/**
 * The selection's verbs in menu order. The selection bar's More menu and a
 * selected row's or card's context menu both render this tree, so they never diverge.
 */
export function SessionSelectionMenuItems({
  verbs,
  kind,
  testId,
}: {
  verbs: SessionSelectionVerbs;
  kind: MenuTreeHost;
  /** Item test ids read `<testId>-<verb>`. */
  testId: string;
}) {
  return <MenuTreeItems host={kind} nodes={selectionMenuTree(verbs, testId, kind)} />;
}

/**
 * A selected row's context menu inside a multi-selection: it names the
 * selection and offers its verbs, never the row's own.
 */
export function SessionSelectionContextItems({
  verbs,
  testId,
}: {
  verbs: SessionSelectionVerbs;
  testId: string;
}) {
  return (
    <ContextMenuGroup data-testid={testId}>
      <ContextMenuLabel data-testid={`${testId}-count`}>
        {sessionCountLabel(verbs.count)}
      </ContextMenuLabel>
      <SessionSelectionMenuItems verbs={verbs} kind="context" testId={testId} />
    </ContextMenuGroup>
  );
}
