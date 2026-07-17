## 0.2.15 - 2026-07-17

### 🎉 Features

- Cy-capture-decisions — skill-only extension for durable decision capture (#237)
### 🐛 Bug Fixes

- Recover stalled and wedged multi-runs (#230)- Share parallel task status enum (#241)- Surface progress and bound the reviews-fix daemon start (#236)- Package cy-qa-workflow as a module and make host.tasks.create v2-aware (#234)- Correct Kiro CLI ACP model handling (#226)- Isolate sync tests and clarify ignore checks (#248)- Isolate task artifacts and add complexity runtime defaults (#250)
### 📚 Documentation

- Add v0.2.15 release highlights

### Release Notes

#### Features

##### Durable architecture decisions
The new cy-capture-decisions skill-only extension reconciles accepted ADRs against the code and review outcome, then promotes only proven, cross-feature decisions into a compact project index with detailed records on demand. The log survives workflow archival and is safe to rerun. (PR #237)

##### Runtime defaults by task complexity
Workspace defaults can now select IDE, model, and reasoning effort for low, medium, high, and critical tasks, with field-wise workspace merging and complexity to type to task-ID precedence while preserving explicit CLI choices. Parallel child snapshots also stop reporting mirrored runtime task artifacts as agent output. (PR #250)

#### Fixes

##### More reliable task and review workflows
QA task injection now supports compozy.tasks/v2 graphs and the cy-qa-workflow extension ships as a verified standalone module. Kiro ACP runs use a valid bootstrap model, slow reviews fix startup is visible and bounded, and parallel task settlement uses one canonical status contract. Test isolation and decision-log ignore guidance were tightened as well. (PRs #234, #226, #236, #241, #248)

#### Highlights

##### Runs that recover instead of hanging
Compozy now detects silent ACP stalls, retries once from a clean worktree, and parks persistent failures with journals and worktrees preserved for triage. Parallel parents can reap wedged children, terminal commands are cancellable and bounded, and the UI reports stalled, retrying, recovered, and parked outcomes. (PR #230)