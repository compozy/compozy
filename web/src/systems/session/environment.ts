/**
 * The session system's environment surface, gathered in one place: where a
 * session runs, the worktree it is bound to, and forking it into a new one.
 *
 * These belong together because they answer one question — which working copy
 * does this session act on — and grouping them keeps the public barrel readable
 * as a surface contract rather than a flat list.
 */
export { SessionEnvironmentField } from "./components/session-environment-field";
export { SessionEnvironmentChip } from "./components/session-environment-chip";
export type { SessionEnvironmentChipState } from "./components/session-environment-chip";
export { SessionEnvironmentControl } from "./components/session-environment-control";
export type { SessionEnvironmentControlHandle } from "./components/session-environment-control";
export { SessionWorktreeForkDialog } from "./components/session-worktree-fork-dialog";
export { SessionWorktreeBindingChip } from "./components/session-worktree-binding-chip";
export { useSessionEnvironment } from "./hooks/use-session-environment";
export type { SessionEnvironmentModel } from "./hooks/use-session-environment";
export {
  useSessionWorktreeBinding,
  WORKTREE_COMMAND_TOKEN,
} from "./hooks/use-session-worktree-binding";
export type { SessionWorktreeBinding } from "./hooks/use-session-worktree-binding";
export { useForkSessionToWorktree } from "./hooks/use-fork-session-to-worktree";
export { forkSessionToWorktree } from "./adapters/session-worktree-api";
export {
  environmentTargetLabel,
  isEnvironmentTargetMissing,
  NEW_WORKTREE_LABEL,
  ROOT_ENVIRONMENT_TARGET,
  selectableWorktrees,
  WORKSPACE_ROOT_LABEL,
} from "./lib/session-environment-target";
export type { SessionEnvironmentTarget } from "./lib/session-environment-target";
