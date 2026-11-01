import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { toast } from "@compozy/ui";

import { useProfileReadScope } from "@/systems/profiles";

import {
  composeMemoryFile,
  countMemoryChanges,
  localDate,
  memoryFullness,
  memoryNoteMatches,
  memoryPercent,
  parseMemoryFile,
  type MemoryFile,
  type MemoryFilter,
  type MemoryNote,
} from "../lib/agent-memory-notes";
import { agentMemoryOptions } from "../lib/query-options";
import type { AgentMemoryDocument, AgentPayload } from "../types";
import {
  useAgentMemoryHistory,
  usePutAgentMemory,
  useRollbackAgentMemory,
  useValidateAgentMemory,
} from "./use-agent-memory";
import { useEditorDocument } from "./use-editor-document";
import { useMinuteClock } from "./use-minute-clock";

const SAVED_COPY = "Saved. It takes effect from the next session.";

function snapshotOf(document: AgentMemoryDocument) {
  return {
    digest: document.digest,
    source: document.content,
    form: parseMemoryFile(document.content),
    block: null,
  };
}

function countChanges(a: MemoryFile, b: MemoryFile) {
  return countMemoryChanges(a.notes, b.notes);
}

/** MEMORY.md as notes: one guarded save, search and filter here, history on the server. */
export function useMemoryEditor(
  agent: AgentPayload,
  workspace: string | null,
  payload: AgentMemoryDocument
) {
  const queryClient = useQueryClient();
  const { destination } = useProfileReadScope();
  const put = usePutAgentMemory();
  const validate = useValidateAgentMemory();
  const rollback = useRollbackAgentMemory();
  const history = useAgentMemoryHistory(agent.name, workspace);
  const scope = { name: agent.name, cacheWorkspace: workspace, profile: destination };

  const document = useEditorDocument({
    resourceKey: JSON.stringify([workspace, agent.name, "memory", destination]),
    snapshot: snapshotOf(payload),
    compose: composeMemoryFile,
    countChanges,
    savedCopy: SAVED_COPY,
    validate: async content => {
      const result = await validate.mutateAsync({ ...scope, params: { content } });
      return result.valid ? undefined : (result.diagnostics[0] ?? "Fix the file to save.");
    },
    write: async (content, expected_digest) =>
      snapshotOf(await put.mutateAsync({ ...scope, params: { content, expected_digest } })),
    reread: async () =>
      snapshotOf(
        await queryClient.fetchQuery({
          ...agentMemoryOptions(agent.name, workspace, destination),
          staleTime: 0,
        })
      ),
  });

  const [adding, setAdding] = useState(false);
  const [seed, setSeed] = useState("");
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<MemoryFilter>("all");
  const today = localDate(useMinuteClock());

  const { form } = document;
  const notes = form.notes;
  const setNotes = (next: readonly MemoryNote[]) => document.setForm({ ...form, notes: next });
  const learned = notes.filter(note => note.learnedIn).length;
  const percent = memoryPercent(payload.budget);
  const fullness = memoryFullness(percent, learned);
  const newestFirst = [...(history.data?.revisions ?? [])].reverse();

  return {
    document,
    budget: payload.budget,
    today,
    notes,
    saved: document.baseline.form.notes,
    adding,
    seed,
    search,
    filter,
    percent,
    fullness,
    full: fullness === "full",
    learned,
    shown: notes.filter(note => memoryNoteMatches(note, filter, search)),
    revisions: newestFirst.map((revision, index) => ({
      id: revision.id,
      name: `Version ${newestFirst.length - index}`,
    })),
    restoring: rollback.isPending,
    setSearch,
    setFilter,
    startAdding: (text = "") => {
      setSeed(text);
      setAdding(true);
    },
    cancelAdding: () => setAdding(false),
    add: (text: string, until?: string) => {
      const note: MemoryNote = {
        id: crypto.randomUUID(),
        text,
        gap: 0,
        ...(until ? { date: today, until } : {}),
      };
      setNotes([note, ...notes]);
      setAdding(false);
      setSeed("");
      setFilter("all");
      setSearch("");
    },
    update: (id: string, text: string) =>
      setNotes(notes.map(note => (note.id === id ? { ...note, text, line: undefined } : note))),
    remove: (id: string) => setNotes(notes.filter(note => note.id !== id)),
    restore: (revision: string) =>
      rollback.mutate(
        { ...scope, params: { revision } },
        { onSuccess: () => toast.message("Back to that version") }
      ),
  };
}

export type MemoryEditorModel = ReturnType<typeof useMemoryEditor>;
