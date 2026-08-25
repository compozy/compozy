import { useMutation, useQueryClient } from "@tanstack/react-query";

import { sessionKeys } from "@/systems/session";

import { exposeSkill, SkillExposeError, unexposeSkill } from "../adapters/skill-api";
import { skillKeys } from "../lib/query-keys";
import { skillExposeResultViews, type SkillExposeResultView } from "../lib/skill-exposure-view";

interface SkillExposeVariables {
  targets: string[];
  workspaceId: string;
}

export interface SkillExposeModel {
  /** Targets currently being written, so the ledger can show them pending. */
  pendingTargets: readonly string[];
  isPending: boolean;
  /** Per-target outcome of the last operation, success or failure alike. */
  results: SkillExposeResultView[];
  /** Sentence for the whole operation when it failed; null otherwise. */
  failure: string | null;
  rolledBack: boolean;
  expose: (targets: string[]) => void;
  unexpose: (targets: string[]) => void;
  dismiss: () => void;
}

/**
 * Expose and unexpose for one skill.
 *
 * Both verbs answer with per-target results, and a failure carries the same
 * shape as a success, so the panel can account for every target it named rather
 * than collapsing the operation into a single error line.
 */
export function useSkillExpose(
  name: string,
  workspaceId: string,
  profile?: string
): SkillExposeModel {
  const queryClient = useQueryClient();

  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: skillKeys.detail(name, workspaceId, profile) }),
      queryClient.invalidateQueries({ queryKey: skillKeys.list(workspaceId, profile) }),
      queryClient.invalidateQueries({ queryKey: sessionKeys.workspaceCommands(workspaceId) }),
    ]);

  const exposeMutation = useMutation({
    mutationFn: ({ targets, workspaceId: id }: SkillExposeVariables) =>
      exposeSkill(name, { targets, ...(id ? { workspace_id: id } : {}) }, profile),
    onSettled: invalidate,
  });
  const unexposeMutation = useMutation({
    mutationFn: ({ targets, workspaceId: id }: SkillExposeVariables) =>
      unexposeSkill(name, { targets, ...(id ? { workspace_id: id } : {}) }, profile),
    onSettled: invalidate,
  });

  const isPending = exposeMutation.isPending || unexposeMutation.isPending;
  const pendingTargets = exposeMutation.isPending
    ? (exposeMutation.variables?.targets ?? [])
    : unexposeMutation.isPending
      ? (unexposeMutation.variables?.targets ?? [])
      : [];
  const last =
    exposeMutation.submittedAt >= unexposeMutation.submittedAt ? exposeMutation : unexposeMutation;
  const failure = last.error instanceof SkillExposeError ? last.error : null;

  return {
    pendingTargets,
    isPending,
    results: skillExposeResultViews(failure?.results ?? last.data?.results ?? []),
    failure: failure?.message ?? null,
    rolledBack: failure?.rolledBack ?? false,
    expose: targets => exposeMutation.mutate({ targets, workspaceId }),
    unexpose: targets => unexposeMutation.mutate({ targets, workspaceId }),
    dismiss: () => {
      exposeMutation.reset();
      unexposeMutation.reset();
    },
  };
}
