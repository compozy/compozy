# Stacked-PR checkpoints (opt-in)

Off by default. Active only when the invocation carried `--stacked` and
bootstrap recorded `state.yaml.stacked: true` (tasks mode only —
`init-state.py` refuses it in free mode). When active, every Phase B
checkpoint publishes its task as one layer of a GitHub stacked-PR chain
instead of piling commits on a single branch. The mechanics live entirely
inside `commit-checkpoint.py`; the loop's phase machine does not change.

## Prerequisites (verify at bootstrap, before the first Phase B iteration)

1. `gh extension list` shows `github/gh-stack`. Missing → repair with
   `gh extension install github/gh-stack`.
2. The repository is enrolled in the stacked-PRs preview: `gh stack init --help`
   exits 0 and a dry `gh stack view` on a stack branch does not report an
   enrollment error. An un-enrolled repo is an **external blocker** for
   stacked mode only — record it and rerun bootstrap without `--stacked`.
3. The loop starts from the branch that will be the stack trunk (usually
   `main`). The first checkpoint captures it as `--base`; do not pre-create a
   feature branch for the loop — layer branches replace it.

## Layer semantics

- **One Phase B task = one layer.** The checkpoint creates
  `<slug>/task-NN` on top of the previous layer and commits there. Stack
  order = execution order emitted by `detect-phase.py`, which already
  linearizes the task graph — fan-out siblings simply stack in the order
  they execute. GitHub reviewers see one task's diff per PR.
- **Phase C/D commits ride the current top layer.** QA fixes and
  peer-review remediation are cross-cutting by nature; they never create
  layers. `--review-round` requires the current branch to already belong to
  the stack.
- **Submission is part of the checkpoint.** After each commit the script runs
  `gh stack submit --auto` (new PRs open as drafts). A clean tree inside a
  stack still re-submits — that heals a commit-ok/submit-failed retry.
- **The loop never merges the stack.** Phase E ends only after required CI is
  green at every open layer's exact head, then prints the done-signature;
  merging (`gh stack merge`, or per-layer) remains the operator's call.

## Resume and drift

On resume, land on the stack top before the next Phase B iteration:
`gh stack checkout <slug>/task-NN` (any layer selects the stack), then
`gh stack top`. The script guards drift: layer branches existing while the
current branch is outside the stack is exit 2 — check out the stack and
retry; never re-init.

## Failure handling

Same repair-loop contract as every checkpoint. `gh stack init/add` failures
happen before the commit (nothing to restore); a `gh stack submit` failure
after a successful commit prints the SHA path on retry via the clean-tree
re-submit. Network/auth failures that survive the repair loop meet the
external-blocker test — record and stop, the local stack is intact.
