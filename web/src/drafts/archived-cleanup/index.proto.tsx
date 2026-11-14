/**
 * Issue #1717: "Clean up history" for the Archived list. A quiet footer verb opens one dialog:
 * what to delete, from when, and a live count of what goes. The count-bearing destructive action
 * is the confirmation; the run reports progress and how it ended.
 */
import { useEffect, useEffectEvent, useState } from "react";

import { toast } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { SESSIONS } from "@/systems/session/mocks/session-list-declutter-fixtures";

import { ArchivedCleanupDialog, type CleanupPhase } from "./archived-cleanup-dialog";
import {
  PROJECT_ARCHIVE,
  SCALE_ARCHIVE,
  SCALE_ARCHIVE_TOTAL,
  archivedRows,
} from "./archived-cleanup-fixtures";
import {
  DEFAULT_CLEANUP_OPTIONS,
  cleanupImpact,
  cleanupRemaining,
  deletedSentence,
  plural,
  type ArchivedThread,
  type CleanupOptions,
} from "./archived-cleanup-model";
import {
  ArchivedCleanupFooter,
  ArchivedFolder,
  ArchivedProjectGroup,
  ArchivedRow,
  ArchivedSidebar,
} from "./archived-sidebar";

export const meta = definePrototype({
  title: "Archived: Clean up history (#1717)",
  route: "/sessions",
  states: [
    { id: "archived-list", label: "1 · Archived list with Clean up history" },
    { id: "active-list", label: "1 · Active list: no entry point" },
    { id: "dialog", label: "2 · Dialog: Everything, 7+ days ago, type to confirm" },
    { id: "dialog-children", label: "3 · Child sessions only, 30+ days ago" },
    { id: "dialog-subagents", label: "3 · Child sessions and subagents, 15+ days ago" },
    { id: "dialog-before", label: "2 · Archived before a chosen date" },
    { id: "dialog-empty", label: "3 · Nothing matches" },
    { id: "deleting", label: "5 · Deleting (keeps going if closed)" },
    { id: "background", label: "5 · Closed while deleting: progress in the footer" },
    { id: "done", label: "5 · Done: list refreshed, toast" },
    { id: "partial", label: "5 · Some sessions weren't deleted" },
    { id: "failed", label: "5 · Nothing was deleted" },
    { id: "scale-list", label: "6 · All projects, 512 archived" },
    { id: "scale", label: "6 · All projects dialog, per project, type to confirm" },
  ],
});

const DAY_MS = 24 * 60 * 60 * 1000;

interface Scenario {
  scope: string;
  archive: readonly ArchivedThread[];
  options?: Partial<CleanupOptions>;
  phase?: CleanupPhase;
  open?: boolean;
  /** Starts right after a successful run: the dialog closed, the toast up. */
  finished?: boolean;
}

/** A project whose archive is all from this week, for the empty preview. */
const QUIET_ARCHIVE = PROJECT_ARCHIVE.flatMap(thread =>
  thread.archivedDaysAgo < 7 ? [{ ...thread, project: "compozy-site" }] : []
);

const ONLY = { everything: false, children: false, subagents: false };

const SCENARIOS: Record<string, Scenario> = {
  "archived-list": { scope: "compozy", archive: PROJECT_ARCHIVE, open: false },
  dialog: { scope: "compozy", archive: PROJECT_ARCHIVE },
  "dialog-children": {
    scope: "compozy",
    archive: PROJECT_ARCHIVE,
    options: { what: { ...ONLY, children: true }, range: "30" },
  },
  "dialog-subagents": {
    scope: "compozy",
    archive: PROJECT_ARCHIVE,
    options: { what: { ...ONLY, children: true, subagents: true }, range: "15" },
  },
  "dialog-before": {
    scope: "compozy",
    archive: PROJECT_ARCHIVE,
    options: { range: "before", before: new Date(Date.now() - 20 * DAY_MS) },
  },
  "dialog-empty": { scope: "compozy-site", archive: QUIET_ARCHIVE },
  deleting: { scope: "compozy", archive: PROJECT_ARCHIVE, phase: { kind: "deleting", done: 64 } },
  background: {
    scope: "compozy",
    archive: PROJECT_ARCHIVE,
    open: false,
    phase: { kind: "deleting", done: 64 },
  },
  done: { scope: "compozy", archive: PROJECT_ARCHIVE, open: false, finished: true },
  partial: {
    scope: "compozy",
    // Everything archived 7+ days ago is 158 sessions; the two threads that failed hold 11.
    archive: PROJECT_ARCHIVE.filter(
      (thread, index) => thread.archivedDaysAgo < 7 || index === 20 || index === 22
    ),
    phase: {
      kind: "partial",
      total: 158,
      deleted: 147,
      failed: [
        {
          title: "Fix #1704 cap reasoning effort levels",
          reason: "It was still stopping. Try again in a moment.",
        },
        {
          title: "Keep Archived and Later views when opening a session",
          reason: "Another window is deleting this session.",
        },
      ],
    },
  },
  failed: {
    scope: "compozy",
    archive: PROJECT_ARCHIVE,
    phase: { kind: "failed", reason: "CompozyOS didn’t answer. Check that it’s running." },
  },
  "scale-list": { scope: "All projects", archive: SCALE_ARCHIVE, open: false },
  scale: { scope: "All projects", archive: SCALE_ARCHIVE, options: { range: "all" } },
};

/** The rows the list shows for an archive: one project, or project groups under All projects. */
function ArchiveRows({ archive, scale }: { archive: readonly ArchivedThread[]; scale: boolean }) {
  if (scale) {
    const rows = archivedRows(
      archive.filter(thread => thread.project === "compozy"),
      9
    );
    return (
      <>
        <ArchivedProjectGroup collapsed={false} count={214} project="compozy">
          <ArchivedFolder count={41} name="Issue batch #1704–#1710" rows={rows.slice(0, 3)} />
          <ArchivedFolder count={18} name="Release v0.7.0" rows={rows.slice(3, 5)} />
          {rows.slice(5).map(session => (
            <ArchivedRow key={session.id} session={session} />
          ))}
        </ArchivedProjectGroup>
        <ArchivedProjectGroup collapsed count={96} project="polygo" />
        <ArchivedProjectGroup collapsed count={71} project="compozy-site" />
        <ArchivedProjectGroup collapsed count={48} project="branas-dashboard" />
        <ArchivedProjectGroup collapsed count={33} project="yoshi" />
        <ArchivedProjectGroup collapsed count={22} project="bible-yt-flow" />
        <ArchivedProjectGroup collapsed count={17} project="releasepr" />
        <ArchivedProjectGroup collapsed count={11} project="dotfiles" />
      </>
    );
  }
  const rows = archivedRows(archive, 14);
  if (rows.length === 0) {
    return <p className="px-3 py-6 text-small-body text-subtle">No archived sessions yet.</p>;
  }
  return (
    <>
      {rows.slice(0, 4).map(session => (
        <ArchivedRow key={session.id} session={session} />
      ))}
      <ArchivedFolder count={3} name="Files preview" rows={rows.slice(4, 7)} />
      {rows.slice(7).map(session => (
        <ArchivedRow key={session.id} session={session} />
      ))}
    </>
  );
}

/**
 * One scenario, live: the footer opens the dialog, confirming runs a short simulated delete, and
 * success closes the dialog onto the refreshed list with a toast, so the whole path can be
 * clicked through from any state.
 */
function CleanupScenario({ initial, scale }: { initial: Scenario; scale: boolean }) {
  const [open, setOpen] = useState(initial.open ?? true);
  const [phase, setPhase] = useState<CleanupPhase>(initial.phase ?? { kind: "form" });
  const [options, setOptions] = useState<Partial<CleanupOptions> | undefined>(initial.options);
  const [archive, setArchive] = useState(initial.archive);
  const [run, setRun] = useState(0);
  const runOptions: CleanupOptions = { ...DEFAULT_CLEANUP_OPTIONS, ...options };
  const deleting = phase.kind === "deleting" ? phase.done : null;
  const runTotal = cleanupImpact(archive, runOptions).total;

  const finish = (from: readonly ArchivedThread[], chosen: CleanupOptions) => {
    toast.success("History cleaned up", {
      description: deletedSentence(cleanupImpact(from, chosen), initial.scope),
    });
    setArchive(cleanupRemaining(from, chosen));
    setOptions(undefined);
    setPhase({ kind: "form" });
    setOpen(false);
  };

  // The "done" state starts where a run just succeeded.
  const finishOnMount = useEffectEvent(() => {
    if (initial.finished) finish(initial.archive, runOptions);
  });
  useEffect(() => {
    const timer = window.setTimeout(finishOnMount, 0);
    return () => window.clearTimeout(timer);
  }, []);

  const advance = useEffectEvent((done: number) => {
    const next = Math.min(runTotal, done + Math.max(1, Math.ceil(runTotal / 8)));
    if (next >= runTotal) finish(archive, runOptions);
    else setPhase({ kind: "deleting", done: next });
  });
  // The daemon keeps deleting whether or not the dialog is open; the seeded deleting states hold
  // still until the person starts a run of their own.
  useEffect(() => {
    if (run === 0 || deleting === null) return;
    const timer = window.setTimeout(() => advance(deleting), 600);
    return () => window.clearTimeout(timer);
  }, [run, deleting]);

  const stop = () => {
    toast("Cleanup stopped", {
      description: `Deleted ${plural(deleting ?? 0, "session", "sessions")} of ${runTotal.toLocaleString()}. The rest are still in Archived.`,
    });
    setOptions(undefined);
    setPhase({ kind: "form" });
    setOpen(false);
  };

  return (
    <main className="flex min-h-dvh bg-canvas">
      <ArchivedSidebar
        footer={
          <ArchivedCleanupFooter
            archivedCount={scale ? SCALE_ARCHIVE_TOTAL : archive.length}
            onCleanUp={() => setOpen(true)}
            progress={deleting === null ? null : { done: deleting, total: runTotal }}
          />
        }
      >
        <ArchiveRows archive={archive} scale={scale} />
      </ArchivedSidebar>
      <ArchivedCleanupDialog
        acrossProjects={scale}
        archive={archive}
        initialOptions={options}
        key={`${run}-${phase.kind}-${open ? "open" : "closed"}`}
        onConfirm={confirmed => {
          setOptions(confirmed);
          setPhase({ kind: "deleting", done: 0 });
          setRun(value => value + 1);
        }}
        onOpenChange={setOpen}
        onRetry={() => finish(archive, runOptions)}
        onStop={stop}
        open={open}
        phase={phase}
        scopeName={initial.scope}
      />
    </main>
  );
}

export default function ArchivedCleanupDraft({ state }: PrototypeProps) {
  if (state === "active-list") {
    return (
      <main className="flex min-h-dvh bg-canvas">
        <ArchivedSidebar shelf="active">
          {[
            SESSIONS.captain,
            SESSIONS.question,
            SESSIONS.chip,
            SESSIONS.board,
            SESSIONS.review,
          ].map(session => (
            <ArchivedRow key={session.id} session={session} />
          ))}
        </ArchivedSidebar>
      </main>
    );
  }
  const scenario = SCENARIOS[state] ?? SCENARIOS["archived-list"]!;
  return (
    <CleanupScenario
      key={state}
      scale={state === "scale" || state === "scale-list"}
      initial={scenario}
    />
  );
}
