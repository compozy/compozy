/**
 * Issue #1717: "Clean up history" for the Archived list, graduated from its draft. The footer verb
 * opens one dialog: what to delete, from when, and the daemon's dry-run count of what goes. The
 * count-bearing destructive action is the confirmation; the run reports progress and how it
 * ended. The draft's whole-sidebar states (Active list without the verb, the success toast, the
 * All projects list) are not component states: the live list reads the daemon.
 */
import { useState } from "react";

import { definePrototype, type PrototypeProps } from "@protohq/cli";

import {
  ARCHIVED_CLEANUP_FIXTURES,
  type ArchivedCleanupFixtureState,
} from "../../mocks/archived-cleanup-fixtures";
import { ArchivedCleanupDialog } from "./archived-cleanup-dialog";
import { ArchivedCleanupFooter } from "./archived-cleanup-footer";

export const meta = definePrototype({
  title: "Archived: Clean up history (#1717)",
  route: "/sessions",
  states: [
    { id: "archived-list", label: "1 · Archived footer: Clean up history" },
    { id: "dialog", label: "2 · Dialog: Everything, 7+ days ago, type to confirm" },
    { id: "dialog-children", label: "3 · Child sessions only, 30+ days ago" },
    { id: "dialog-subagents", label: "3 · Child sessions and subagents, 15+ days ago" },
    { id: "dialog-before", label: "2 · Archived before a chosen date" },
    { id: "dialog-empty", label: "3 · Nothing matches" },
    { id: "deleting", label: "5 · Deleting (keeps going if closed)" },
    { id: "background", label: "5 · Closed while deleting: progress in the footer" },
    { id: "partial", label: "5 · Some sessions weren't deleted" },
    { id: "failed", label: "5 · Nothing was deleted" },
    { id: "scale", label: "6 · All projects dialog, per project, type to confirm" },
  ],
});

const noop = () => undefined;

function CleanupState({ state }: { state: ArchivedCleanupFixtureState }) {
  const fixture = ARCHIVED_CLEANUP_FIXTURES[state];
  const closed = "closed" in fixture && fixture.closed === true;
  const [open, setOpen] = useState(!closed);
  const [options, setOptions] = useState(fixture.options);
  const deleting = fixture.phase.kind === "deleting" ? fixture.phase : null;
  return (
    <>
      <ArchivedCleanupFooter
        archivedCount={fixture.archivedCount}
        onCleanUp={() => setOpen(true)}
        progress={deleting ? { done: deleting.done, total: deleting.total } : null}
      />
      <ArchivedCleanupDialog
        acrossProjects={"acrossProjects" in fixture && fixture.acrossProjects === true}
        counts={fixture.counts}
        impact={fixture.impact}
        onConfirm={noop}
        onOpenChange={setOpen}
        onOptionsChange={setOptions}
        onRetry={noop}
        onStop={noop}
        open={open}
        options={options}
        phase={fixture.phase}
        scopeName={fixture.scopeName}
      />
    </>
  );
}

export default function ArchivedCleanupPrototype({ state }: PrototypeProps) {
  const known = state in ARCHIVED_CLEANUP_FIXTURES;
  const id = (known ? state : "archived-list") as ArchivedCleanupFixtureState;
  return <CleanupState key={id} state={id} />;
}
