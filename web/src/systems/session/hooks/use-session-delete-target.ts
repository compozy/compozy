import { useRef, useState } from "react";
import type { QueryClient } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";

import { reportActionError } from "./session-lifecycle-helpers";
import {
  prepareSessionDeleteTarget,
  type PreparedSessionDeleteTarget,
} from "./session-delete-preparation";
import { sessionsRunningProcesses } from "../lib/session-processes";
import { readSessionTrees, sessionChildrenChoice, sessionChildrenScope } from "../lib/session-tree";
import type { SessionPayload } from "../types";

export type SessionDeleteTarget = PreparedSessionDeleteTarget & {
  bulk: boolean;
  open: boolean;
  treeReadPending: boolean;
  onSelectionChange?: (ids: readonly string[]) => void;
};

/** Owns the frozen delete target and its explicit, user-triggered tree retry. */
export function useSessionDeleteTarget(queryClient: QueryClient, computerId: ComputerId) {
  const [target, setTarget] = useState<SessionDeleteTarget | null>(null);
  const [included, setIncluded] = useState(false);
  const generation = useRef(0);
  const preparing = useRef(false);
  const retrying = useRef<number | undefined>(undefined);
  const childrenScope = target?.childrenScope ?? sessionChildrenScope("delete", [], new Map());
  const childrenPending = target !== null && (target.treeReadPending || target.treeErrors.size > 0);

  const prepare = (
    sessions: readonly SessionPayload[],
    bulk: boolean,
    onSelectionChange?: (ids: readonly string[]) => void
  ) => {
    if (sessions.length === 0 || preparing.current || target?.open) return;
    preparing.current = true;
    const request = ++generation.current;
    setIncluded(false);
    void prepareSessionDeleteTarget(queryClient, sessions, bulk, computerId)
      .then(prepared => {
        if (request !== generation.current) return;
        setTarget({
          ...prepared,
          bulk,
          open: true,
          treeReadPending: false,
          ...(onSelectionChange ? { onSelectionChange } : {}),
        });
      })
      .catch(error => {
        if (request === generation.current) reportActionError("delete", error);
      })
      .finally(() => {
        if (request === generation.current) preparing.current = false;
      });
  };

  const retryTrees = () => {
    const current = target;
    if (
      !current?.open ||
      current.treeReadPending ||
      current.treeErrors.size === 0 ||
      retrying.current !== undefined
    )
      return;
    const request = ++generation.current;
    retrying.current = request;
    const failed = current.sessions.filter(session => current.treeErrors.has(session.id));
    setTarget(value => (value?.open ? { ...value, treeReadPending: true } : value));
    void readSessionTrees(queryClient, failed)
      .then(({ trees, errors }) => {
        if (request !== generation.current) return;
        setTarget(value => {
          if (!value?.open) return value;
          const nextTrees = new Map(value.trees);
          for (const [id, tree] of trees) nextTrees.set(id, tree);
          const nextErrors = new Map(value.treeErrors);
          for (const session of failed) nextErrors.delete(session.id);
          for (const [id, error] of errors) nextErrors.set(id, error);
          return {
            ...value,
            trees: nextTrees,
            treeErrors: nextErrors,
            childrenScope: sessionChildrenScope("delete", value.sessions, nextTrees),
            treeReadPending: false,
          };
        });
      })
      .catch(error => {
        if (request !== generation.current) return;
        setTarget(value => {
          if (!value?.open) return value;
          const nextErrors = new Map(value.treeErrors);
          for (const session of failed) nextErrors.set(session.id, error);
          return { ...value, treeErrors: nextErrors, treeReadPending: false };
        });
      })
      .finally(() => {
        if (retrying.current === request) retrying.current = undefined;
      });
  };

  const close = () => {
    generation.current++;
    retrying.current = undefined;
    setTarget(value => (value ? { ...value, open: false } : value));
  };
  const updateChildrenScope = (sessions: readonly SessionPayload[]) =>
    setTarget(value =>
      value
        ? {
            ...value,
            childrenScope: sessionChildrenScope("delete", sessions, value.trees),
            processCount: sessionsRunningProcesses(sessions),
          }
        : value
    );
  const childSessions = target
    ? sessionChildrenChoice(childrenScope, included, setIncluded, {
        pending: childrenPending,
        onRetry: target.treeErrors.size > 0 ? retryTrees : undefined,
      })
    : undefined;

  return {
    target,
    setTarget,
    included,
    setIncluded,
    prepare,
    close,
    updateChildrenScope,
    childrenPending,
    childrenScope,
    childSessions,
    includedTrees: included && childrenScope.members.length > 0 ? target?.trees : undefined,
  };
}
