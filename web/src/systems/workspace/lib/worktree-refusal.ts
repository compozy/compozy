import { WorktreeApiError } from "../adapters/worktree-api";

/**
 * Canonical daemon error codes (TechSpec §Deterministic error codes). The design
 * artboards quote older hyphenated spellings; these are the ones the runtime
 * actually emits.
 */
export const WORKTREE_ERROR_CODES = {
  nameTaken: "worktree_name_taken",
  pathExists: "worktree_path_exists",
  branchHeld: "branch_held_by_worktree",
  branchAtRoot: "branch_checked_out_at_root",
  baseRefNotFound: "base_ref_not_found",
  repoHasNoCommits: "repo_has_no_commits",
  dirtyRequiresForce: "worktree_dirty_requires_force",
  unpushedRequiresForce: "worktree_unpushed_requires_force",
  sessionActive: "worktree_session_active",
  adoptionMainCheckout: "adoption_main_checkout",
  adoptionForeignRepository: "adoption_foreign_repository",
  adoptionUnreadable: "adoption_unreadable",
} as const;

/** Quantified loss for a removal refusal. Rendered verbatim, never rounded. */
export interface WorktreeRemovalRisk {
  changedFiles: number;
  insertions: number;
  deletions: number;
  unpushedCommits: number;
  /** The commits also live on a remote, so the local loss is recoverable. */
  existsOnRemote: boolean;
}

export interface WorktreeRefusal {
  code: string;
  message: string;
  /** Present on removal refusals. */
  risk: WorktreeRemovalRisk | null;
  /** Branch also exists on a remote, so the local loss is recoverable. */
  downgrade: boolean;
  /** Named holder for `branch_held_by_worktree`. */
  details: Record<string, string> | null;
}

function readRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function readNumber(source: Record<string, unknown> | null, key: string): number {
  const value = source?.[key];
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function readStringDetails(value: unknown): Record<string, string> | null {
  const record = readRecord(value);
  if (!record) return null;
  const details: Record<string, string> = {};
  for (const [key, entry] of Object.entries(record)) {
    if (typeof entry === "string") details[key] = entry;
  }
  return Object.keys(details).length > 0 ? details : null;
}

function readRisk(body: Record<string, unknown>): WorktreeRemovalRisk | null {
  const risk = readRecord(body.risk);
  if (!risk) return null;
  return {
    changedFiles: readNumber(risk, "changed_files"),
    insertions: readNumber(risk, "insertions"),
    deletions: readNumber(risk, "deletions"),
    unpushedCommits: readNumber(risk, "unpushed_commits"),
    existsOnRemote: risk.exists_on_remote === true,
  };
}

/**
 * The single decode point for worktree failures — dialogs consume the result and
 * never re-parse a response body. Returns `null` when the error is not a
 * structured daemon refusal, so callers fall back to the generic message.
 */
export function decodeWorktreeRefusal(error: unknown): WorktreeRefusal | null {
  if (!(error instanceof WorktreeApiError)) return null;

  const body = readRecord(error.body);
  if (!body) return null;
  const code = typeof body.code === "string" ? body.code.trim() : "";
  if (code === "") return null;

  return {
    code,
    message: refusalMessage(
      code,
      typeof body.error === "string" && body.error.trim() !== "" ? body.error : error.message
    ),
    risk: readRisk(body),
    downgrade: body.downgrade === true,
    details: readStringDetails(body.details),
  };
}

/** The branch-held refusal names the worktree already holding the branch. */
export function refusalBranchHolder(refusal: WorktreeRefusal | null): string | null {
  if (refusal?.code !== WORKTREE_ERROR_CODES.branchHeld) return null;
  const holder =
    refusal.details?.worktree_path ?? refusal.details?.worktree ?? refusal.details?.holder;
  return holder?.trim() ? holder : null;
}

function refusalMessage(code: string, fallback: string): string {
  switch (code) {
    case WORKTREE_ERROR_CODES.nameTaken:
      return "A worktree with this name already exists.";
    case WORKTREE_ERROR_CODES.pathExists:
      return "This path is already in use.";
    case WORKTREE_ERROR_CODES.branchHeld:
      return "This branch is already checked out in another worktree.";
    case WORKTREE_ERROR_CODES.branchAtRoot:
      return "This branch is checked out in the workspace root.";
    case WORKTREE_ERROR_CODES.baseRefNotFound:
      return "The base ref could not be found.";
    case WORKTREE_ERROR_CODES.repoHasNoCommits:
      return "Create the repository's first commit before adding a worktree.";
    case WORKTREE_ERROR_CODES.dirtyRequiresForce:
    case WORKTREE_ERROR_CODES.unpushedRequiresForce:
      return "Removing this worktree would discard local work.";
    case WORKTREE_ERROR_CODES.sessionActive:
      return "A session is running a turn on this worktree.";
    case WORKTREE_ERROR_CODES.adoptionMainCheckout:
      return "Its metadata resolves into the main checkout, not a linked worktree. Nothing was changed.";
    case WORKTREE_ERROR_CODES.adoptionForeignRepository:
      return "Its metadata belongs to a different repository. Nothing was changed.";
    case WORKTREE_ERROR_CODES.adoptionUnreadable:
      return "Its linked-worktree metadata could not be verified. Nothing was changed.";
    default:
      return fallback;
  }
}
